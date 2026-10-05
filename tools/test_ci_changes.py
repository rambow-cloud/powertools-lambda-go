"""Exercise routing, real Git history, and the exact always-running CI gate."""

from copy import deepcopy
import json
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
        for path in ("README.md", "logger/README.md", "parameters/testdata/README.md", "integration/dynamodblocal/README.md", "docs/LOGGER.md", "docs/assets/logo.png", "website/check_guides.py", "mkdocs.yml"):
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

    def test_nested_ownership_and_test_only_scope(self):
        for path, directory in (("eventhandler/http/metrics/metrics_test.go", "eventhandler/http/metrics"),
                                ("commons/dynamodb/value_test.go", "commons/dynamodb"),
                                ("parameters/testdata/aws-http-scenarios.json", "parameters"),
                                ("parameters/http_scenarios_test.go", "parameters")):
            with self.subTest(path=path):
                selection = ci_changes.select([path])
                self.assertEqual(selection["modules"], [directory])
                self.assertFalse(selection["full"])
                self.assertFalse(selection["runtime"])

    def test_production_source_and_dependency_changes_include_consumers(self):
        for path in ("parameters/parameters.go", "parameters/go.mod", "parameters/go.sum"):
            with self.subTest(path=path):
                selection = ci_changes.select([path])
                self.assertIn("parameters", selection["modules"])
                self.assertIn("integration", selection["modules"])
                self.assertTrue(selection["dynamodb"])
                self.assertFalse(selection["runtime"])
                self.assertFalse(selection["full"])
        self.assertNotIn("logger", ci_changes.select(["parameters/parameters.go"])["modules"])

    def test_transitive_dependencies_and_older_versions(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "tools").mkdir()
            entries = [{"directory": name, "version": "v0.2.0"} for name in (".", "child", "nested", "old")]
            (root / "tools/modules.json").write_text(json.dumps({"base": "example.com/project", "modules": entries}))
            for name, body in ((".", ""), ("child", "require example.com/project v0.2.0\n"),
                               ("nested", "require (\n example.com/project/child v0.2.0 // indirect\n)\n"),
                               ("old", "require example.com/project v0.1.0\n")):
                (root / name).mkdir(exist_ok=True)
                module = "example.com/project" + ("/" + name if name != "." else "")
                (root / name / "go.mod").write_text("module " + module + "\ngo 1.26\n" + body)
            self.assertEqual(ci_changes.select(["identity.go"], root=root)["modules"], [".", "child", "nested"])
            self.assertEqual(ci_changes.select(["identity_test.go"], root=root)["modules"], ["."])

    def test_service_and_runtime_jobs_are_independently_scoped(self):
        for path in ("integration/dynamodblocal/service_test.go", "integration/dynamodblocal/scenarios.json", "integration/local/dynamodb_run.py", "integration/local/test_dynamodb_run.py"):
            selection = ci_changes.select([path])
            self.assertEqual(selection["modules"], ["integration"])
            self.assertTrue(selection["dynamodb"])
            self.assertFalse(selection["runtime"])
            self.assertFalse(selection["full"])
        for path in ("integration/internal/kmsfixture/fixture.go", "integration/local/run.py", "tools/reference/package-lock.json"):
            self.assertTrue(ci_changes.select([path])["runtime"])
        for path in ("README.md", ".github/labels.json", "logger/logger_test.go"):
            self.assertFalse(ci_changes.select([path])["dynamodb"])
        self.assertTrue(ci_changes.select(["integration/go.sum"])["dynamodb"])
        self.assertTrue(ci_changes.select(["integration/go.mod"])["runtime"])

    def test_release_and_shared_inputs_select_every_module_and_suite(self):
        directories = sorted(ci_changes.module_graph(ROOT)[0])
        for path in ("releases/new.json", "tools/modules.json", "go.mod", "go.work", "tools/modules.py", "tools/package/main.go", "new-fixture.bin", "new-module/go.mod"):
            selection = ci_changes.select([path])
            self.assertEqual(selection["modules"], directories)
            self.assertTrue(selection["full"])
            self.assertTrue(all(selection[name] for name in (*ci_changes.SUITES, "runtime", "dynamodb")))
        self.assertEqual(ci_changes.select(["README.md"], force=True)["modules"], directories)

    def test_modules_job_uses_structured_scoped_arguments_and_full_mode(self):
        step = next(item for item in WORKFLOW["jobs"]["test"]["steps"] if item.get("name") == "Test, vet, tidy-check, and verify independent consumers")
        script = step["run"].split("\n", 1)[1].rstrip().removesuffix("\nPY")
        base = ["uv", "run", "--no-project", "python", "tools/modules.py", "check"]
        for full, expected in (("true", base), ("false", base + ["--only", "parameters", "--only", "logger"])):
            with patch.dict(os.environ, CI_MODULES='["parameters", "logger"]', CI_FULL=full), patch.object(subprocess, "run") as run:
                exec(script, {})
                run.assert_called_once_with(expected, check=True)
        for modules, full in (("[]", "false"), ('["parameters; echo injected"]', "false"), ('[null]', "true"), ('{}', 'true'), ('["logger"]', '')):
            with patch.dict(os.environ, CI_MODULES=modules, CI_FULL=full), patch.object(subprocess, "run") as run:
                with self.assertRaises(ValueError):
                    exec(script, {})
                run.assert_not_called()

    def test_lambda_artifacts_are_reserved_for_full_regression(self):
        steps = WORKFLOW["jobs"]["test"]["steps"]
        for name in ("Cross-compile Lambda for x86_64", "Cross-compile Lambda for arm64", "Validate static ELF binaries and package executable bootstrap ZIPs", "Upload Lambda example artifacts"):
            self.assertEqual(next(item for item in steps if item.get("name") == name)["if"], "needs.changes.outputs.full == 'true'")
        self.assertEqual(WORKFLOW["jobs"]["runtime"]["if"], "needs.changes.outputs.runtime == 'true'")
        self.assertEqual(WORKFLOW["jobs"]["dynamodb"]["if"], "needs.changes.outputs.dynamodb == 'true'")
        self.assertEqual(set(WORKFLOW["jobs"]["runtime"]["needs"]), {"changes", "test"})

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
            "changes": {"result": "success", "outputs": {"go": "false", "docs": "true", "automation": "true", "release": "false", "runtime": "false", "dynamodb": "false", "full": "false", "modules": "[]"}},
            "test": {"result": "skipped"}, "runtime": {"result": "skipped"}, "dynamodb": {"result": "skipped"},
            "docs": {"result": "success"}, "automation": {"result": "success"},
        }

    def full_needs(self):
        needs = self.needs()
        needs["changes"]["outputs"].update(dict.fromkeys((*ci_changes.SUITES, "runtime", "dynamodb", "full"), "true"), modules='["."]')
        for job in ("test", "runtime", "dynamodb"):
            needs[job]["result"] = "success"
        return needs

    def test_fast_path_and_full_path_pass(self):
        NAMESPACE["validate"](self.needs())
        NAMESPACE["validate"](self.full_needs())
        scoped = self.needs()
        scoped["changes"]["outputs"].update(go="true", modules='["parameters"]', dynamodb="true")
        scoped["test"]["result"] = scoped["dynamodb"]["result"] = "success"
        NAMESPACE["validate"](scoped)

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
        needs = self.full_needs()
        for job in ("test", "runtime", "dynamodb", "docs", "automation"):
            for result in ("failure", "cancelled", "skipped"):
                broken = deepcopy(needs)
                broken[job]["result"] = result
                with self.subTest(job=job, result=result), self.assertRaises(ValueError):
                    NAMESPACE["validate"](broken)

    def test_invalid_module_service_and_full_selections_fail_closed(self):
        for modules in ("null", "{}", '[null]', '[""]', '["logger", "logger"]', '["logger"]', 'not-json'):
            needs = self.needs()
            needs["changes"]["outputs"]["modules"] = modules
            with self.subTest(modules=modules), self.assertRaises(ValueError):
                NAMESPACE["validate"](needs)
        for suite in ("runtime", "dynamodb", "full"):
            needs = self.needs()
            needs["changes"]["outputs"][suite] = "true"
            with self.subTest(suite=suite), self.assertRaises(ValueError):
                NAMESPACE["validate"](needs)
        for suite in ("runtime", "dynamodb", "full", "modules"):
            needs = self.needs()
            del needs["changes"]["outputs"][suite]
            with self.subTest(missing=suite), self.assertRaises(ValueError):
                NAMESPACE["validate"](needs)

    def test_unselected_job_cannot_claim_success(self):
        needs = self.needs()
        needs["runtime"]["result"] = "success"
        with self.assertRaises(ValueError):
            NAMESPACE["validate"](needs)

    def test_required_gate_always_runs_and_covers_all_suites(self):
        gate = WORKFLOW["jobs"]["gate"]
        self.assertEqual(gate["if"], "always()")
        self.assertEqual(set(gate["needs"]), {"changes", "test", "runtime", "dynamodb", "docs", "automation"})
        events = WORKFLOW.get("on", WORKFLOW.get(True))
        self.assertNotIn("paths", events["pull_request"] or {})
        self.assertNotIn("paths", events["push"])
        self.assertEqual(WORKFLOW["env"]["CGO_ENABLED"], "0")

    def test_full_regression_marker_depends_on_the_gate_and_checks_failures(self):
        full = WORKFLOW["jobs"]["full"]
        self.assertEqual(full["name"], "Full regression")
        self.assertEqual(set(full["needs"]), {"changes", "gate"})
        self.assertEqual(full["if"], "always() && needs.changes.outputs.full == 'true'")
        script = full["steps"][0]["run"].split("\n", 1)[1].rstrip().removesuffix("\nPY")
        for change, gate in (("success", "success"), ("failure", "success"), ("success", "skipped"), ("success", "failure"), ("success", "cancelled")):
            with patch.dict(os.environ, CHANGE_RESULT=change, GATE_RESULT=gate):
                if change == gate == "success":
                    exec(script, {})
                else:
                    with self.assertRaises(ValueError):
                        exec(script, {})


if __name__ == "__main__":
    unittest.main(verbosity=2)
