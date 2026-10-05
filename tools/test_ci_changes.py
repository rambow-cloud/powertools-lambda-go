"""Exercise routing, real Git history, and the exact always-running CI gate."""

from copy import deepcopy
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import yaml

import ci_changes


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = yaml.safe_load((ROOT / ".github/workflows/ci.yml").read_text())
SCRIPT = WORKFLOW["jobs"]["gate"]["steps"][0]["run"]
NAMESPACE = {"__name__": "ci_gate"}
exec(compile(SCRIPT.removeprefix("python3 - <<'PY'\n").rstrip().removesuffix("\nPY"), "ci.yml", "exec"), NAMESPACE)


class RoutingTests(unittest.TestCase):
    def suites(self, *paths):
        return {name for name, enabled in ci_changes.classify(paths).items() if enabled}

    def test_docs_do_not_run_go_or_release_tools(self):
        for path in ("README.md", "logger/README.md", "docs/LOGGER.md", "docs/assets/logo.png", "website/check_guides.py", "mkdocs.yml"):
            with self.subTest(path=path):
                self.assertEqual(self.suites(path), {"docs"})

    def test_metadata_fast_path(self):
        for path in (".github/ISSUE_TEMPLATE/bug_report.yml", ".github/labels.json", ".github/PULL_REQUEST_TEMPLATE.md", "tools/test_contribution_workflow.py"):
            with self.subTest(path=path):
                self.assertEqual(self.suites(path), {"automation"})

    def test_workflows_run_their_own_checks(self):
        self.assertEqual(self.suites(".github/workflows/docs.yml"), {"docs", "automation"})
        self.assertEqual(self.suites(".github/workflows/release.yml"), {"automation", "release"})
        self.assertEqual(self.suites(".github/workflows/automation.yml"), {"automation", "release"})
        self.assertEqual(self.suites("tools/release.py"), {"automation", "release"})
        self.assertEqual(self.suites("website/uv.lock"), {"docs", "automation", "release"})
        self.assertEqual(self.suites(".github/workflows/ci.yml"), set(ci_changes.SUITES))

    def test_runtime_inputs_cannot_take_the_metadata_fast_path(self):
        for path in ("logger/logger.go", "logger/go.sum", "go.work", "integration/local/compose.yml", "integration/fixtures/event.json", "tools/reference/package-lock.json", "tools/modules.py", "tools/test_runtime_ci.py", "new-fixture.bin", "LICENSE"):
            with self.subTest(path=path):
                self.assertIn("go", self.suites(path))

    def test_mixed_changes_union_and_release_batches_are_full(self):
        self.assertEqual(self.suites("docs/LOGGER.md", ".github/labels.json"), {"docs", "automation"})
        for path in ("releases/auto-example.json", "tools/modules.json", "tools/ci_changes.py", "tools/test_ci_changes.py"):
            self.assertEqual(self.suites(path), set(ci_changes.SUITES))

    def test_manual_new_branch_and_empty_diff_are_full(self):
        self.assertTrue(all(ci_changes.classify([]).values()))
        self.assertTrue(all(ci_changes.classify(["README.md"], force=True).values()))
        self.assertEqual(ci_changes.changed_paths({}, "workflow_dispatch"), ([], True))
        self.assertEqual(ci_changes.changed_paths({"before": "0" * 40, "after": "a" * 40}, "push"), ([], True))

    def test_invalid_or_unavailable_base_fails_instead_of_skipping(self):
        with self.assertRaises(ValueError):
            ci_changes.changed_paths({"before": "--help", "after": "a" * 40}, "push")
        with patch.object(ci_changes.subprocess, "run", side_effect=subprocess.CalledProcessError(128, "git")):
            with self.assertRaises(subprocess.CalledProcessError):
                ci_changes.changed_paths({"before": "b" * 40, "after": "a" * 40}, "push")

    def test_real_git_multicommit_rename_deletion_and_pr_merge_base(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def git(*args):
                return subprocess.check_output(["git", "-C", directory, *args], text=True).strip()
            git("init", "--quiet")
            git("config", "user.name", "CI fixture")
            git("config", "user.email", "fixture@example.invalid")
            (root / "old.go").write_text("package fixture\n")
            (root / "removed.go").write_text("package fixture\n")
            git("add", ".")
            git("commit", "--quiet", "-m", "base")
            base = git("rev-parse", "HEAD")
            git("mv", "old.go", "renamed with spaces.md")
            git("rm", "removed.go")
            git("commit", "--quiet", "-m", "rename and delete")
            (root / "README.md").write_text("Docs\n")
            git("add", ".")
            git("commit", "--quiet", "-m", "second commit")
            head = git("rev-parse", "HEAD")
            git("checkout", "--quiet", "--detach", base)
            (root / "base-only.go").write_text("package fixture\n")
            git("add", ".")
            git("commit", "--quiet", "-m", "base advanced")
            advanced = git("rev-parse", "HEAD")
            original = os.getcwd()
            try:
                os.chdir(directory)
                push, _ = ci_changes.changed_paths({"before": base, "after": head}, "push")
                pr, _ = ci_changes.changed_paths({"pull_request": {"base": {"sha": advanced}, "head": {"sha": head}}}, "pull_request")
            finally:
                os.chdir(original)
            expected = {"old.go", "removed.go", "renamed with spaces.md", "README.md"}
            self.assertEqual(set(push), expected)
            self.assertEqual(set(pr), expected)
            self.assertIn("go", self.suites(*pr))


class GateTests(unittest.TestCase):
    def needs(self):
        return {
            "changes": {"result": "success", "outputs": {"go": "false", "docs": "true", "automation": "true", "release": "false"}},
            "test": {"result": "skipped"}, "runtime": {"result": "skipped"},
            "docs": {"result": "success"}, "automation": {"result": "success"},
        }

    def test_fast_path_and_full_path_pass(self):
        NAMESPACE["validate"](self.needs())
        needs = self.needs()
        needs["changes"]["outputs"] = dict.fromkeys(ci_changes.SUITES, "true")
        needs["test"]["result"] = needs["runtime"]["result"] = "success"
        NAMESPACE["validate"](needs)

    def test_selection_failure_missing_output_and_inconsistent_release_fail(self):
        needs = self.needs()
        for result in ("failure", "cancelled", "skipped"):
            needs["changes"]["result"] = result
            with self.assertRaises(ValueError):
                NAMESPACE["validate"](needs)
        needs = self.needs()
        del needs["changes"]["outputs"]["go"]
        with self.assertRaises(ValueError):
            NAMESPACE["validate"](needs)
        needs = self.needs()
        needs["changes"]["outputs"].update(release="true", automation="false")
        with self.assertRaises(ValueError):
            NAMESPACE["validate"](needs)

    def test_selected_failure_cancellation_or_skip_blocks_gate(self):
        needs = self.needs()
        needs["changes"]["outputs"] = dict.fromkeys(ci_changes.SUITES, "true")
        needs["test"]["result"] = needs["runtime"]["result"] = "success"
        for job in ("test", "runtime", "docs", "automation"):
            for result in ("failure", "cancelled", "skipped"):
                broken = deepcopy(needs)
                broken[job]["result"] = result
                with self.subTest(job=job, result=result), self.assertRaises(ValueError):
                    NAMESPACE["validate"](broken)

    def test_required_gate_always_runs_and_covers_all_suites(self):
        gate = WORKFLOW["jobs"]["gate"]
        self.assertEqual(gate["if"], "always()")
        self.assertEqual(set(gate["needs"]), {"changes", "test", "runtime", "docs", "automation"})
        events = WORKFLOW.get("on", WORKFLOW.get(True))
        self.assertNotIn("paths", events["pull_request"] or {})
        self.assertNotIn("paths", events["push"])
        self.assertEqual(WORKFLOW["env"]["CGO_ENABLED"], "0")


if __name__ == "__main__":
    unittest.main(verbosity=2)
