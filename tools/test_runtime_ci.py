"""Offline regressions for runtime CI evidence; no Docker or AWS calls."""

from copy import deepcopy
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "integration/local"))
import ci_report
import run as runtime


class RuntimeEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.identity = {"source_sha": "a" * 40, "run_id": "101", "run_attempt": "1"}
        self.rie = self.root / "dist/local/report.json"
        self.stream = self.root / "dist/local/streaming/report.json"
        self.batch = self.root / "dist/runtime-acceptance/BATCH_ACCEPTANCE.json"
        self.kms = self.root / "dist/runtime-acceptance/DATAMASKING_KMS_INTEROP.json"
        report = {"execution": self.identity, "passed": True, "completed": True,
                  "checks": [{"passed": True}], "runtime_architecture": "amd64",
                  "binary_architectures": ["amd64", "arm64"], "cgo_enabled": False,
                  "builds_executed": True, "go_validation_executed": False, "cleanup_errors": []}
        for path in (self.rie, self.stream, self.batch, self.kms):
            self.write(path, deepcopy(report))
        for arch in ("amd64", "arm64"):
            for name in ("bootstrap", "stream-bootstrap"):
                (self.root / f"dist/local/{arch}").mkdir(parents=True, exist_ok=True)
                (self.root / f"dist/local/{arch}/{name}").write_bytes(b"synthetic executable")
        for name in ("capture", "kmsinterop"):
            (self.root / f"dist/local/amd64/{name}").write_bytes(b"synthetic executable")
        for name in ("otlp.json", "lambda.log"):
            (self.root / "dist/local" / name).write_text("synthetic evidence", encoding="utf-8")
        self.bind_batch()

    def write(self, path, data):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(data), encoding="utf-8")

    def update(self, path, **changes):
        data = json.loads(path.read_text(encoding="utf-8"))
        data.update(changes)
        self.write(path, data)

    def bind_batch(self):
        self.update(self.batch, sha256={name: ci_report.digest(self.root / "dist/local" / name)
                    for name in ("report.json", "otlp.json", "lambda.log")})

    def test_current_complete_evidence_passes_and_records_binaries(self):
        summary = ci_report.summarize(self.root, self.identity)
        self.assertTrue(summary["passed"], summary["errors"])
        self.assertEqual(len(summary["binary_sha256"]), 6)
        self.assertEqual(sum(item["assertions"] for item in summary["suites"].values()), 4)
        self.assertIn(self.identity["source_sha"], ci_report.render(summary))

    def test_missing_report_does_not_fall_back_to_documented_history(self):
        self.kms.unlink()
        self.write(self.root / "docs/DATAMASKING_KMS_INTEROP.json", {"completed": True})
        summary = ci_report.summarize(self.root, self.identity)
        self.assertFalse(summary["passed"])
        self.assertFalse(summary["suites"]["Go to TypeScript KMS"]["passed"])

    def test_other_commits_runs_and_attempts_are_rejected(self):
        for key, value in (("source_sha", "b" * 40), ("run_id", "102"), ("run_attempt", "2")):
            with self.subTest(key=key):
                self.update(self.kms, execution={**self.identity, key: value})
                self.assertFalse(ci_report.summarize(self.root, self.identity)["passed"])

    def test_failed_and_empty_checks_cannot_hide_behind_success_flag(self):
        for checks in ([], [{"passed": False}], [{"passed": "true"}]):
            with self.subTest(checks=checks):
                self.update(self.kms, checks=checks)
                self.assertFalse(ci_report.summarize(self.root, self.identity)["passed"])

    def test_cleanup_failure_is_fatal_even_when_assertions_pass(self):
        self.update(self.stream, cleanup_errors=["synthetic removal failure"])
        self.assertFalse(ci_report.summarize(self.root, self.identity)["passed"])

    def test_reused_binaries_cannot_be_claimed_as_fresh_builds(self):
        self.update(self.rie, builds_executed=False)
        self.bind_batch()
        self.assertFalse(ci_report.summarize(self.root, self.identity)["passed"])

    def test_batch_evidence_must_match_current_logs(self):
        (self.root / "dist/local/lambda.log").write_text("changed evidence", encoding="utf-8")
        self.assertFalse(ci_report.summarize(self.root, self.identity)["passed"])

    def test_missing_arm64_build_is_fatal(self):
        (self.root / "dist/local/arm64/stream-bootstrap").unlink()
        self.assertFalse(ci_report.summarize(self.root, self.identity)["passed"])

    def test_missing_ci_identity_cannot_validate_local_history(self):
        self.assertFalse(ci_report.summarize(self.root, {})["passed"])

    def test_acceptance_output_redirect_preserves_local_default(self):
        with patch.object(runtime, "ROOT", self.root), patch.dict(os.environ, {}, clear=True):
            self.assertEqual(runtime.acceptance_path("report.json"), self.root / "docs/report.json")
            with patch.dict(os.environ, {"POWERTOOLS_ACCEPTANCE_DIR": str(self.root / "dist/evidence")}):
                self.assertEqual(runtime.acceptance_path("report.json"), self.root / "dist/evidence/report.json")


if __name__ == "__main__":
    unittest.main(verbosity=2)
