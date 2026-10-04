"""Offline automation regressions using isolated Git repositories and Go metadata."""

from contextlib import contextmanager
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import yaml

import release
import release_automation as completion
import release_prepare as preparation


ROOT = Path(__file__).resolve().parents[1]
BASE = "github.com/" + release.REPOSITORY


def native_runs(sha, conclusion="success"):
    return [{"id": number, "path": ".github/workflows/" + filename, "event": "pull_request", "head_sha": sha,
             "head_repository": {"full_name": release.REPOSITORY}, "pull_requests": [{"number": 21}],
             "status": "completed", "conclusion": conclusion}
            for number, filename in enumerate(("contribution.yml", "ci.yml", "docs.yml"), 101)]


class API:
    def __init__(self):
        self.releases, self.tags, self.prs, self.files, self.associations = [], [], {}, {}, {}
        self.writes = []
        self.open_prs = []
        self.issue_state = "open"
        self.native = None
        self.pr_head = None

    def request(self, path, **kwargs):
        return {"login": "maintainer"}

    def repo(self, path, **kwargs):
        if kwargs.get("method", "GET") != "GET":
            self.writes.append((path, kwargs["data"]))
            if path == "issues":
                return {"number": 20, "state": "open"}
            if path == "pulls":
                sha = self.pr_head or release.git("rev-parse", "HEAD")
                self.native = native_runs(sha, "action_required")
                return {"number": 21, "html_url": "https://example.com/pull/21", "head": {"sha": sha}}
            return None
        if path.startswith("collaborators/"):
            return {"permission": "admin"}
        if path.startswith("issues/"):
            return {"number": 20, "state": self.issue_state}
        if path.startswith("pulls/"):
            return self.prs[int(path.split("/")[1])]
        if path.startswith("git/ref/heads/"):
            return None
        if path.startswith("actions/runs?"):
            sha = path.split("head_sha=", 1)[1].split("&", 1)[0]
            return {"workflow_runs": self.native if self.native is not None else native_runs(sha)}
        if path.startswith("commits/") and "/check-runs?" in path:
            return {"check_runs": [{"id": number, "name": name, "app": {"slug": "github-actions"}, "status": "completed", "conclusion": "success"} for number, name in enumerate(("PR contribution policy", "Modules and Lambda artifacts", "Build documentation"))]}
        if path.startswith("git/ref/tags/"):
            sha = self.tag_shas.get(path.removeprefix("git/ref/tags/"))
            return {"object": {"type": "commit", "sha": sha}} if sha else None
        raise AssertionError(path)

    def pages(self, path):
        if path == "releases":
            return iter(self.releases)
        if path == "tags":
            return iter({"name": tag} for tag in self.tag_shas)
        if path.startswith("commits/"):
            return iter(self.associations.get(path.split("/")[1], []))
        if path.startswith("pulls/"):
            return iter(self.files.get(int(path.split("/")[1]), []))
        if path.startswith("pulls?"):
            return iter(self.open_prs)
        raise AssertionError(path)

    tag_shas = {}


def command(root, *args):
    result = subprocess.run(args, cwd=root, env={**os.environ, "CGO_ENABLED": "0"}, text=True, encoding="utf-8", capture_output=True, check=True)
    return result.stdout.strip()


@contextmanager
def fixture():
    (ROOT / ".tmp").mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="release-automation-", dir=ROOT / ".tmp") as temporary:
        root = Path(temporary)
        modules = [
            {"directory": ".", "version": "v0.1.0", "public": True, "package": "commons"},
            {"directory": "logger", "version": "v0.1.0", "public": True},
            {"directory": "metrics", "version": "v0.1.0", "public": True},
            {"directory": "examples", "version": "v0.1.0", "public": False},
            {"directory": "tracer/xray", "version": "v0.1.0", "public": True, "status": "deprecated-frozen"},
            {"directory": "tools", "version": "v0.1.0", "public": False},
        ]
        for module in modules:
            directory = root / module["directory"]
            directory.mkdir(parents=True, exist_ok=True)
            name = BASE + ("" if module["directory"] == "." else "/" + module["directory"])
            required = BASE + "/logger" if module["directory"] == "examples" else BASE
            dependency = f"\nrequire {required} v0.1.0\n" if module["directory"] not in {".", "tools"} else ""
            (directory / "go.mod").write_text(f"module {name}\n\ngo 1.26\n{dependency}", encoding="utf-8")
            (directory / "LICENSE").write_text("Synthetic offline fixture license.\n", encoding="utf-8")
            (directory / "NOTICE").write_text("Synthetic offline fixture notice.\n", encoding="utf-8")
            if module["directory"] == ".":
                (directory / "commons").mkdir()
                (directory / "commons/identity.go").write_text("package commons\n\nconst Value = 1\n", encoding="utf-8")
            elif module["directory"] != "tools":
                package = module["directory"].rsplit("/", 1)[-1]
                imported = required if package == "examples" else BASE + "/commons"
                identifier = "logger" if package == "examples" else "commons"
                (directory / "value.go").write_text(f'package {package}\n\nimport "{imported}"\n\nconst Value = {identifier}.Value\n', encoding="utf-8")
        (root / "tools/modules.json").write_text(json.dumps({"base": BASE, "modules": modules}), encoding="utf-8")
        shutil.copyfile(ROOT / "tools/modules.py", root / "tools/modules.py")
        (root / "go.work").write_text("go 1.26\n\nuse (\n" + "\n".join("\t" + ("." if module["directory"] == "." else "./" + module["directory"]) for module in modules) + "\n)\n", encoding="utf-8")
        (root / ".gitignore").write_text("dist/\n__pycache__/\n", encoding="utf-8")
        # Detached Git maintenance must not outlive a disposable fixture.
        for args in (("init",), ("config", "maintenance.auto", "false"), ("config", "gc.auto", "0"),
                     ("config", "user.name", "Fixture"), ("config", "user.email", "fixture@example.com"),
                     ("add", "."), ("commit", "-m", "Import modules")):
            command(root, "git", *args)
        source = command(root, "git", "rev-parse", "HEAD")
        command(root, "git", "update-ref", "refs/remotes/origin/main", source)
        api = API()
        api.tag_shas = {}
        with patch.object(release, "ROOT", root), patch.dict(os.environ, {"GITHUB_ACTOR": "maintainer"}):
            yield root, api, source


def args(**changes):
    return SimpleNamespace(**{**dict(module=["logger"], all=False, issue=20, plan="fixture", overrides=None, bump="auto", local=True, auto_publish=False), **changes})


class VersionTests(unittest.TestCase):
    def test_automatic_and_explicit_version_policy(self):
        cases = [
            ("v0.1.0", None, "auto", [], (), "v0.1.0"),
            ("v0.1.0", "logger/v0.1.0", "auto", [{"type": "fix"}], (), "v0.1.1"),
            ("v0.1.0", "logger/v0.1.0", "auto", [{"type": "feature"}], (), "v0.2.0"),
            ("v0.1.0", "logger/v0.1.0", "auto", [{"type": "breaking"}], (), "v0.2.0"),
            ("v0.1.0", "logger/v0.1.0", "major", [], (), "v1.0.0"),
            ("v0.1.0", None, "auto", [], ("v0.1.0", "v0.1.1"), "v0.1.2"),
            ("v0.1.0", "logger/v0.1.0", "minor", [], ("v0.2.0-rc.1",), "v0.2.0"),
        ]
        for current, previous, bump, entries, reserved, expected in cases:
            with self.subTest(expected=expected):
                self.assertEqual(preparation.next_version(current, previous, bump, entries, reserved), expected)
        with self.assertRaisesRegex(ValueError, "module-path migration"):
            preparation.next_version("v1.0.0", "logger/v1.0.0", "auto", [{"type": "breaking"}])

    def test_all_excludes_development_and_frozen_modules(self):
        with fixture():
            modules = release.manifest()
            self.assertEqual(preparation.scope_modules(modules, all_modules=True), [".", "logger", "metrics"])
            for selected in (["tracer/xray"], ["examples"], ["logger", "logger"], ["../escape"]):
                with self.subTest(selected=selected), self.assertRaises(ValueError):
                    preparation.scope_modules(modules, selected)
            self.assertEqual(preparation.affected_modules(["logger/value.go", "commons/identity.go", ".github/workflows/ci.yml"], modules), [".", "logger", "repository"])


class PreparationTests(unittest.TestCase):
    def test_first_component_release_includes_unpublished_dependencies_and_generates_history(self):
        with fixture() as (root, api, source):
            preparation.prepare(args(auto_publish=True), api)
            plan = json.loads((root / "releases/fixture.json").read_text())
            self.assertEqual(plan["requested_modules"], ["logger"])
            self.assertEqual(plan["included_dependencies"], ["."])
            self.assertEqual({item["directory"]: item["version"] for item in plan["modules"]}, {"logger": "v0.1.0", ".": "v0.1.0"})
            self.assertTrue(plan["auto_publish"])
            self.assertTrue(all(item["initial_summary"] for item in plan["modules"]))
            self.assertEqual(plan["modules"][0]["untracked_commits"][source]["description"], "Import modules")
            release.validate_plan(plan, release.manifest())
            self.assertEqual(api.writes, [])

    def test_patch_release_synchronizes_go_mod_go_work_and_development_consumers(self):
        with fixture() as (root, api, source):
            for tag in ("v0.1.0", "logger/v0.1.0"):
                command(root, "git", "tag", tag)
                api.tag_shas[tag] = source
                api.releases.append({"tag_name": tag, "draft": False, "prerelease": False})
            (root / "logger/fix.go").write_text("package logger\n\nconst Fixed = true\n", encoding="utf-8")
            command(root, "git", "add", ".")
            command(root, "git", "commit", "-m", "fix(logger): fix fields")
            target = command(root, "git", "rev-parse", "HEAD")
            command(root, "git", "update-ref", "refs/remotes/origin/main", target)
            pr = {"number": 11, "title": "fix(logger): fix fields", "body": "## Release notes\n\n- logger | fix | Preserve temporary fields.\n", "merged_at": "2026-10-04T00:00:00Z", "merge_commit_sha": target, "base": {"ref": "main", "repo": {"full_name": release.REPOSITORY}}}
            api.associations[target] = [pr]
            preparation.prepare(args(), api)
            plan = json.loads((root / "releases/fixture.json").read_text())
            self.assertEqual([item["directory"] for item in plan["modules"]], ["logger"])
            self.assertEqual(plan["modules"][0]["version"], "v0.1.1")
            self.assertIn("pull/11", plan["modules"][0]["notes"])
            self.assertIn(BASE + "/logger v0.1.1", (root / "examples/go.mod").read_text())
            workspace = json.loads(command(root, "go", "work", "edit", "-json", str(root / "go.work")))
            mappings = {(item["Old"]["Path"], item["Old"].get("Version")): item["New"]["Path"] for item in workspace["Replace"]}
            self.assertIn((BASE + "/logger", "v0.1.1"), mappings)
            self.assertEqual(release.manifest()["metrics"]["version"], "v0.1.0")
            self.assertIn("examples", plan["metadata_updates"])

    def test_all_prepares_every_maintained_component(self):
        with fixture() as (root, api, _):
            preparation.prepare(args(module=None, all=True), api)
            plan = json.loads((root / "releases/fixture.json").read_text())
            self.assertEqual({item["directory"] for item in plan["modules"]}, {".", "logger", "metrics"})
            self.assertEqual(plan["included_dependencies"], [])

    def test_failed_tidy_restores_original_metadata_without_github_writes(self):
        with fixture() as (root, api, _):
            originals = {path: path.read_bytes() for path in root.rglob("*") if path.is_file() and (path.name in {"go.mod", "go.sum", "go.work", "go.work.sum", "modules.json"})}
            original_run = release.run
            def run(*command, **kwargs):
                if command[:2] == (sys.executable, str(root / "tools/modules.py")):
                    raise RuntimeError("Synthetic tidy failure")
                return original_run(*command, **kwargs)
            with patch.object(release, "run", side_effect=run), self.assertRaisesRegex(RuntimeError, "tidy failure"):
                preparation.prepare(args(), api)
            self.assertEqual({path: path.read_bytes() for path in originals}, originals)
            self.assertEqual(command(root, "git", "status", "--porcelain"), "")
            self.assertEqual(api.writes, [])

    def test_historical_pr_scope_is_inferred_but_malformed_current_notes_fail(self):
        with fixture() as (_, api, source):
            pr = {"number": 11, "title": "fix(logger): repair output", "body": "## Summary\n\nRepair output.", "changed_files": 1, "merged_at": "2026-10-04T00:00:00Z", "merge_commit_sha": source, "base": {"ref": "main", "repo": {"full_name": release.REPOSITORY}}}
            api.associations[source] = [pr]
            api.prs[11] = pr
            api.files[11] = [{"filename": "logger/value.go"}]
            entries, reviewed, direct, generated = preparation.History(api, release.manifest(), source, {}).notes(None)
            self.assertEqual(entries, [{"module": "logger", "type": "fix", "description": pr["title"], "pr": 11}])
            self.assertEqual(reviewed, [11])
            self.assertEqual(direct, {})
            self.assertEqual(generated[0]["source"], "PR title and changed module paths")
            pr["body"] = "## Release notes\n\n- logger | nonsense | Invalid."
            with self.assertRaises(ValueError):
                preparation.History(api, release.manifest(), source, {}).notes(None)

    def test_tracking_issue_pr_and_required_checks_are_created_automatically(self):
        with fixture() as (root, api, source):
            original_git = release.git
            def git(*command):
                if command[:2] == ("-c", "credential.helper="):
                    self.assertIn("push", command)
                    return ""
                return original_git(*command)
            with patch.object(release, "git", side_effect=git):
                preparation.prepare(args(issue=None, local=False, auto_publish=True), api)
            self.assertEqual([path for path, _ in api.writes], ["issues", "pulls", "actions/runs/101/approve", "actions/runs/102/approve", "actions/runs/103/approve"])
            pr = api.writes[1][1]
            self.assertEqual(pr["base"], "main")
            self.assertIn("Refs #20", pr["body"])
            self.assertIn("**enabled**", pr["body"])
            self.assertEqual(api.writes[2][1], {})
            self.assertEqual(command(root, "git", "rev-parse", "HEAD^1"), source)
            self.assertEqual(command(root, "git", "status", "--porcelain"), "")

    def test_unpublished_dependencies_are_expanded_transitively(self):
        modules = {directory: {"public": True} for directory in ("logger", "metrics", ".")}
        requirements = {"logger": [{"Path": BASE + "/metrics", "Version": "v0.1.0"}], "metrics": [{"Path": BASE, "Version": "v0.1.0"}], ".": []}
        self.assertEqual(preparation.select_dependencies(["logger"], modules, requirements, lambda tag: False), ["logger", "metrics", "."])
        self.assertEqual(preparation.select_dependencies(["logger"], modules, requirements, lambda tag: tag == "v0.1.0"), ["logger", "metrics"])

    def test_pushed_preparation_branch_recovers_after_pr_creation_failure(self):
        with fixture() as (root, api, source):
            original_git, original_repo = release.git, api.repo
            saved = {}
            def git(*arguments):
                if arguments[:2] == ("-c", "credential.helper="):
                    return ""
                return original_git(*arguments)
            def repo(path, **kwargs):
                if path.startswith("git/ref/heads/") and saved:
                    return {"object": {"sha": saved["sha"]}}
                result = original_repo(path, **kwargs)
                if path == "pulls" and not saved:
                    saved["sha"] = command(root, "git", "rev-parse", "HEAD")
                    api.pr_head = saved["sha"]
                    raise RuntimeError("Synthetic PR creation failure")
                return result
            with patch.object(release, "git", side_effect=git), patch.object(api, "repo", side_effect=repo):
                with self.assertRaisesRegex(RuntimeError, "PR creation failure"):
                    preparation.prepare(args(issue=None, local=False, auto_publish=True), api)
                command(root, "git", "checkout", "--detach", source)
                with patch.object(preparation, "module_requirements") as regenerate:
                    preparation.prepare(args(issue=None, local=False, auto_publish=True), api)
                regenerate.assert_not_called()
            self.assertEqual(sum(path == "issues" for path, _ in api.writes), 1)
            self.assertEqual(sum(path == "pulls" for path, _ in api.writes), 2)
            self.assertEqual(command(root, "git", "status", "--porcelain"), "")

    def test_duplicate_request_reuses_preparation_pr_without_writes(self):
        with fixture() as (_, api, _):
            api.open_prs = [{"number": 21, "head": {"sha": "b" * 40}, "html_url": "https://example.com/pull/21"}]
            with patch.object(preparation, "module_requirements") as metadata:
                preparation.prepare(args(local=False), api)
            metadata.assert_not_called()
            self.assertEqual(api.writes, [])


class NativeCheckTests(unittest.TestCase):
    pr = {"number": 21, "head": {"sha": "b" * 40}}

    def test_only_this_pr_head_and_native_events_are_eligible(self):
        for mismatch in ("event", "head_sha", "repository", "pr"):
            api = API()
            api.native = native_runs(self.pr["head"]["sha"])
            invalid = api.native[0]
            if mismatch == "repository":
                invalid["head_repository"]["full_name"] = "external/fork"
            elif mismatch == "pr":
                invalid["pull_requests"][0]["number"] = 22
            else:
                invalid[mismatch] = "workflow_dispatch" if mismatch == "event" else "a" * 40
            with self.subTest(mismatch=mismatch), self.assertRaisesRegex(ValueError, "Native PR checks have not appeared"):
                preparation.start_pr_checks(self.pr, api, wait_seconds=0)
            self.assertEqual(api.writes, [])

    def test_successful_and_pending_runs_are_retained(self):
        api = API()
        api.native = native_runs(self.pr["head"]["sha"])
        for index, status in ((1, "in_progress"), (2, "queued")):
            api.native[index].update(status=status, conclusion=None)
        preparation.start_pr_checks(self.pr, api, resume=True)
        self.assertEqual(api.writes, [])

    def test_approval_and_failure_recovery_keep_native_event_association(self):
        api = API()
        api.native = native_runs(self.pr["head"]["sha"])
        api.native[0]["conclusion"] = "action_required"
        api.native[1]["conclusion"] = "failure"
        api.native.append({**api.native[2], "id": 50, "conclusion": "failure"})
        preparation.start_pr_checks(self.pr, api, resume=True)
        self.assertEqual([path for path, _ in api.writes], ["actions/runs/101/approve", "actions/runs/102/rerun"])

    def test_asynchronous_native_run_creation_is_awaited(self):
        api = API()
        responses = [{"workflow_runs": []}, {"workflow_runs": native_runs(self.pr["head"]["sha"])}]
        with patch.object(api, "repo", side_effect=responses), patch.object(preparation.time, "sleep") as sleep:
            preparation.start_pr_checks(self.pr, api)
        sleep.assert_called_once_with(2)

    def test_forbidden_approval_reports_the_actual_maintainer_action(self):
        api = API()
        api.native = native_runs(self.pr["head"]["sha"], "action_required")
        original = api.repo
        def repo(path, **kwargs):
            if path.endswith("/approve"):
                raise RuntimeError("GitHub request failed (HTTP 403)")
            return original(path, **kwargs)
        with patch.object(api, "repo", side_effect=repo), self.assertRaisesRegex(ValueError, "Approve workflows to run"):
            preparation.start_pr_checks(self.pr, api)
        self.assertEqual(api.writes, [])


class CompletionTests(unittest.TestCase):
    def merged(self, root, api, source, automatic=True):
        plan = {"schema_version": 1, "issue": 20, "source_sha": source, "auto_publish": automatic, "modules": []}
        (root / "releases").mkdir()
        (root / "releases/fixture.json").write_text(json.dumps(plan), encoding="utf-8")
        command(root, "git", "add", ".")
        command(root, "git", "commit", "-m", "Prepare release")
        sha = command(root, "git", "rev-parse", "HEAD")
        command(root, "git", "update-ref", "refs/remotes/origin/main", sha)
        api.prs[21] = {"number": 21, "merged": True, "merged_at": "2026-10-04T00:00:00Z", "merge_commit_sha": sha, "base": {"ref": "main", "repo": {"full_name": release.REPOSITORY}}}
        api.files[21] = [{"filename": "releases/fixture.json", "status": "added"}]
        api.associations[sha] = [api.prs[21]]
        event = {"workflow_run": {"event": "push", "head_branch": "main", "head_sha": sha, "head_repository": {"full_name": release.REPOSITORY}}}
        return event

    def test_publication_inputs_are_resolved_from_one_merged_pr(self):
        with fixture() as (root, api, source):
            event = self.merged(root, api, source)
            resolved, _ = completion.resolve_preparation(21, api)
            self.assertEqual((resolved.issue, resolved.plan, resolved.sha), (20, "fixture", event["workflow_run"]["head_sha"]))
            api.prs[21]["merged"] = False
            with self.assertRaises(ValueError):
                completion.resolve_preparation(21, api)

    def test_auto_publication_requires_plan_authorization_and_completed_main_checks(self):
        for scenario in ("ready", "pending", "manual", "closed", "fork", "pr-event", "wrong-branch"):
            with self.subTest(scenario=scenario), fixture() as (root, api, source):
                event = self.merged(root, api, source, automatic=scenario != "manual")
                if scenario == "fork":
                    event["workflow_run"]["head_repository"]["full_name"] = "external/fork"
                if scenario == "pr-event":
                    event["workflow_run"]["event"] = "pull_request"
                if scenario == "wrong-branch":
                    event["workflow_run"]["head_branch"] = "feature"
                if scenario == "closed":
                    api.issue_state = "closed"
                with patch.object(release, "check_runs", side_effect=ValueError("Pending") if scenario == "pending" else None) as checks:
                    result = completion.automatic_preparation(event, api)
                if scenario == "ready":
                    self.assertTrue(result.publish)
                    checks.assert_called_once()
                else:
                    self.assertIsNone(result)
                self.assertEqual(api.writes, [])


class WorkflowTests(unittest.TestCase):
    def test_preparation_and_completion_need_no_additional_secret(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/prepare-release.yml").read_text())
        events = workflow.get("on", workflow.get(True))
        self.assertEqual(set(events), {"workflow_dispatch"})
        self.assertEqual(set(events["workflow_dispatch"]["inputs"]), {"scope", "bump", "auto_publish"})
        self.assertTrue(events["workflow_dispatch"]["inputs"]["auto_publish"]["default"])
        self.assertFalse(workflow["concurrency"]["cancel-in-progress"])
        self.assertEqual(workflow["jobs"]["prepare"]["permissions"]["actions"], "write")
        for step in workflow["jobs"]["prepare"]["steps"]:
            if "run" in step:
                self.assertNotIn("${{", step["run"])
        self.assertNotIn("secrets.", json.dumps(workflow))


if __name__ == "__main__":
    unittest.main(verbosity=2)
