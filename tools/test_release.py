"""Offline release regressions; never create public tags or Releases."""

import json
from contextlib import ExitStack
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import threading
import unittest
from unittest.mock import patch
from urllib.parse import unquote, urlsplit

import yaml

import release
from release_notes import parse_notes, render_notes, validate_direct_note

ROOT = Path(__file__).resolve().parents[1]
A, B, C = "a" * 40, "b" * 40, "c" * 40
MODULES = {
    ".": {"public": True, "version": "v0.1.0", "package": "commons"},
    "logger": {"public": True, "version": "v0.1.1"},
    "metrics": {"public": True, "version": "v0.1.0"},
    "examples": {"public": False, "version": "v0.1.0"},
    "tracer/xray": {"public": True, "version": "v0.1.0", "status": "deprecated-frozen"},
}


def entry(module="logger", kind="fix", description="Preserve field lifetimes.", pr=10):
    return {"module": module, "type": kind, "description": description, "pr": pr}


def item(directory="logger", version="v0.1.1", previous="logger/v0.1.0"):
    entries = [entry(module=directory)]
    initial = "Initial supported scope; complete parity is not promised." if previous is None else None
    return {"directory": directory, "version": version, "previous_tag": previous, "entries": entries, "initial_summary": initial, "untracked_commits": {}, "reviewed_prs": [10], "excluded_prs": [], "notes": render_notes(release.REPOSITORY, directory, version, previous, entries, initial)}


def plan(items=None):
    return {"schema_version": 1, "issue": 20, "source_sha": A, "modules": items if items is not None else [item()]}


def pr(number, sha, base="main"):
    return {"number": number, "merged_at": f"2026-10-{number:02}T00:00:00Z", "merge_commit_sha": sha, "base": {"ref": base, "repo": {"full_name": release.REPOSITORY}}}


class FakeAPI:
    def __init__(self, responses=None):
        self.token = "test-token"
        self.responses = responses or {}
        self.calls = []

    def repo(self, path, **kwargs):
        self.calls.append((path, kwargs))
        if kwargs.get("method", "GET") != "GET":
            return {"id": 1, "draft": True, "html_url": "https://example.com/release"} if path == "releases" else {}
        value = self.responses.get(path)
        return None if path.startswith("releases/tags/") and value and value.get("draft") else value

    def pages(self, path):
        self.calls.append((path, {}))
        return iter(self.responses.get(path, []))

    def request(self, path, **kwargs):
        return {"login": "maintainer"}


class NotesTests(unittest.TestCase):
    def test_multiple_entries_and_modules(self):
        notes = parse_notes("## Release notes\n\n- logger | fix | Fix children.\n- metrics | feature | Add options.\n- logger | fix | Fix buffers.\n", MODULES)
        self.assertEqual([record["module"] for record in notes], ["logger", "metrics", "logger"])

    def test_missing_notes_unknown_modules_and_examples_fail(self):
        self.assertEqual(parse_notes("## Release notes\n\nNone: Tests only.", MODULES), [])
        for text in ("", "## Release notes\n\nNone:", "## Release notes\n\n<!-- - logger | fix | Example. -->", "## Release notes\n\n```\n- logger | fix | Example.\n```", "## Release notes\n\n- unknown | fix | Example.", "## Release notes\n\n- logger | unknown | Example."):
            with self.subTest(text=text), self.assertRaises(ValueError):
                parse_notes(text, MODULES)

    def test_accumulated_notes_keep_pr_links_and_exclude_other_modules(self):
        entries = [entry(pr=10), entry(kind="feature", pr=11), entry("metrics", pr=12), entry(description="Fix buffers.", pr=13), entry(kind="breaking", pr=14)]
        notes = render_notes(release.REPOSITORY, "logger", "v0.1.1", "logger/v0.1.0", entries)
        for number in (10, 11, 13, 14):
            self.assertIn(f"/pull/{number}", notes)
        self.assertNotIn("/pull/12", notes)
        self.assertLess(notes.index("## Breaking changes"), notes.index("## Features"))
        self.assertIn("logger/v0.1.0...logger/v0.1.1", notes)

    def test_first_release_and_direct_commits_are_explicit(self):
        with self.assertRaisesRegex(ValueError, "initial scope"):
            render_notes(release.REPOSITORY, ".", "v0.1.0", None, [])
        notes = render_notes(release.REPOSITORY, ".", "v0.1.0", None, [], "Shared core; documented limits apply.", {A: {"modules": ["."], "description": "Initial source import."}, B: {"modules": ["repository"], "description": "Website changes."}})
        self.assertIn("## Initial release", notes)
        self.assertIn(f"/commit/{A}", notes)
        self.assertNotIn(f"/commit/{B}", notes)
        self.assertIn("powertools-lambda-go@v0.1.0", notes)

    def test_pr_text_is_data(self):
        value = "$(touch marker) `code` | preserve text"
        self.assertEqual(parse_notes(f"## Release notes\n\n- logger | fix | {value}", MODULES)[0]["description"], value)

    def test_direct_commits_require_reviewed_module_scope(self):
        validate_direct_note({"modules": [".", "logger"], "description": "Initial sources."}, MODULES)
        for note in ("No scope", {}, {"modules": ["unknown"], "description": "Change."}, {"modules": ["logger", "logger"], "description": "Change."}):
            with self.subTest(note=note), self.assertRaises(ValueError):
                validate_direct_note(note, MODULES)


class HistoryTests(unittest.TestCase):
    def test_commit_range_deduplicates_merge_and_rebase_associations(self):
        first, second, outside, already_released = pr(10, B), pr(11, C), pr(12, "d" * 40), pr(14, A)
        api = FakeAPI({f"commits/{B}/pulls": [first, outside, already_released], f"commits/{C}/pulls": [first, second, pr(13, C, base="other")]})
        with patch.object(release, "git", side_effect=[f"{B}\n{C}", f"{B}\n{C}"]) as git:
            prs, uncovered = release.merged_prs(api, A, C)
        self.assertEqual([record["number"] for record in prs], [10, 11])
        self.assertEqual(uncovered, [])
        self.assertEqual(git.call_args_list[0].args, ("rev-list", "--first-parent", "--reverse", C, "^" + A))
        self.assertEqual(git.call_args_list[1].args, ("rev-list", C, "^" + A))

    def test_initial_history_exposes_untracked_commits(self):
        with patch.object(release, "git", side_effect=[A, A]):
            self.assertEqual(release.merged_prs(FakeAPI(), None, A), ([], [A]))

    def test_history_cache_reuses_github_results(self):
        api = FakeAPI({f"commits/{B}/pulls": [pr(10, B)]})
        cache = {}
        with patch.object(release, "git", side_effect=[B, B, B, B]):
            release.merged_prs(api, None, B, cache)
            release.merged_prs(api, None, B, cache)
        self.assertEqual(len(api.calls), 1)

    def test_previous_release_uses_module_ancestry_and_stable_channel(self):
        releases = [{"tag_name": tag, "draft": draft, "prerelease": prerelease} for tag, draft, prerelease in [("logger/v0.1.0", False, False), ("metrics/v0.9.0", False, False), ("logger/v0.2.0-rc.1", False, True), ("logger/v0.3.0", True, False), ("logger/v0.4.0", False, False)]]
        with patch.object(release, "commit_of_tag", side_effect=lambda tag: tag), patch.object(release, "is_ancestor", side_effect=lambda tag, target: "v0.4.0" not in tag):
            self.assertEqual(release.previous_release("logger", C, releases), "logger/v0.1.0")
            self.assertEqual(release.previous_release("logger", C, releases, prerelease=True), "logger/v0.2.0-rc.1")

    def test_github_pagination(self):
        api = object.__new__(release.GitHub)
        api.repo = lambda path: list(range(100)) if path.endswith("&page=1") else [100]
        self.assertEqual(len(list(api.pages("releases"))), 101)


class PlanTests(unittest.TestCase):
    def test_frozen_notes_and_manifest_versions(self):
        self.assertEqual(list(release.validate_plan(plan(), MODULES)), ["logger"])
        for change in (lambda data: data["modules"][0].update(version="v0.2.0"), lambda data: data["modules"][0].update(notes="Unreviewed text."), lambda data: data["modules"].append(data["modules"][0]), lambda data: data.update(source_sha="main"), lambda data: data["modules"][0].update(reviewed_prs=[])):
            data = plan()
            change(data)
            with self.assertRaises(ValueError):
                release.validate_plan(data, MODULES)

    def test_frozen_and_development_modules_fail(self):
        for directory in ("examples", "tracer/xray"):
            with self.subTest(directory=directory), self.assertRaises(ValueError):
                release.validate_plan(plan([item(directory, "v0.1.0", None)]), MODULES)

    def test_dependencies_publish_first_or_must_already_exist(self):
        selected = {"logger": item(), ".": item(".", "v0.1.0", None)}
        requirements = {"logger": [{"Path": "github.com/" + release.REPOSITORY, "Version": "v0.1.0"}], ".": []}
        self.assertEqual(release.dependency_order(selected, MODULES, requirements, lambda tag: False), [".", "logger"])
        with self.assertRaisesRegex(ValueError, "Include"):
            release.dependency_order({"logger": item()}, MODULES, requirements, lambda tag: False)
        self.assertEqual(release.dependency_order({"logger": item()}, MODULES, requirements, lambda tag: True), ["logger"])
        requirements["."] = [{"Path": "github.com/" + release.REPOSITORY + "/logger", "Version": "v0.1.1"}]
        with self.assertRaisesRegex(ValueError, "cycle"):
            release.dependency_order(selected, MODULES, requirements, lambda tag: False)

    def test_versions_and_unsafe_paths(self):
        self.assertLess(release.version_key("v0.1.0-rc.2"), release.version_key("v0.1.0-rc.10"))
        self.assertLess(release.version_key("v0.1.0-rc.10"), release.version_key("v0.1.0"))
        for version in ("v01.1.0", "v2.0.0", "v0.1.0-01", "v0.1.0+metadata"):
            with self.subTest(version=version), self.assertRaises(ValueError):
                release.version_key(version)
        for name in ("../other", "x/y", "$(command)", "Uppercase"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                release.plan_path(name)


class PublicationTests(unittest.TestCase):
    def args(self, publish=False):
        return type("Args", (), {"issue": 20, "pr": 21, "sha": B, "plan": "logger-update", "publish": publish})()

    def context(self, temporary, data, api=None):
        Path(temporary, ".goreleaser.json").write_bytes((ROOT / ".goreleaser.json").read_bytes())
        stack = ExitStack()
        stack.enter_context(patch.object(release, "preflight", return_value=(data, {"logger": data["modules"][0]}, MODULES, ["logger"])))
        stack.enter_context(patch.object(release, "run", return_value="GitVersion:    " + release.GORELEASER_VERSION))
        stack.enter_context(patch.object(release, "git", return_value=""))
        def cli(directory, selected, args, output, config, token=None, draft=True):
            if token and api:
                record = {"tag_name": "logger/v0.1.1", "draft": draft, "html_url": "https://example.com/release", "name": "logger/v0.1.1", "body": selected["notes"], "prerelease": False}
                api.responses["releases/tags/logger%2Fv0.1.1"] = record
                api.responses["releases"] = [record]
        stack.enter_context(patch.object(release, "run_goreleaser", side_effect=cli))
        return stack

    def test_preflight_never_writes_github_or_runs_unpublished_consumers(self):
        data, api = plan(), FakeAPI()
        with tempfile.TemporaryDirectory() as temporary, patch.object(release, "ROOT", Path(temporary)), self.context(temporary, data), patch.object(release, "verify_consumer") as verify:
            release.write_json(release.plan_path("logger-update"), data)
            release.publish(self.args(), api)
            result = json.loads((Path(temporary) / "dist/releases/logger-update/progress.json").read_text())
        self.assertTrue(result["completed"])
        verify.assert_not_called()
        self.assertEqual(api.calls, [])

    def test_consumer_failure_keeps_tag_and_draft_and_issue_open(self):
        data, api = plan(), FakeAPI()
        with tempfile.TemporaryDirectory() as temporary, patch.object(release, "ROOT", Path(temporary)), self.context(temporary, data, api), patch.object(release, "remote_tag", return_value=None), patch.object(release, "verify_consumer", side_effect=RuntimeError("Public proxy not ready.")):
            release.write_json(release.plan_path("logger-update"), data)
            with self.assertRaisesRegex(RuntimeError, "proxy"):
                release.publish(self.args(True), api)
            result = json.loads((Path(temporary) / "dist/releases/logger-update/progress.json").read_text())
        writes = [(path, values) for path, values in api.calls if values.get("method", "GET") != "GET"]
        self.assertEqual([path for path, _ in writes], ["git/refs"])
        self.assertTrue(api.responses["releases/tags/logger%2Fv0.1.1"]["draft"])
        self.assertFalse(result["completed"])

    def test_goreleaser_creates_draft_then_publishes_after_consumer(self):
        data, api = plan(), FakeAPI()
        with tempfile.TemporaryDirectory() as temporary, patch.object(release, "ROOT", Path(temporary)), self.context(temporary, data, api), patch.object(release, "remote_tag", return_value=None):
            release.write_json(release.plan_path("logger-update"), data)
            def consumer(*args):
                self.assertTrue(api.responses["releases/tags/logger%2Fv0.1.1"]["draft"])
                return {"sum": "h1:verified"}
            with patch.object(release, "verify_consumer", side_effect=consumer):
                release.publish(self.args(True), api)
        self.assertFalse(api.responses["releases/tags/logger%2Fv0.1.1"]["draft"])
        self.assertEqual([path for path, values in api.calls if values.get("method", "GET") != "GET"], ["git/refs", "issues/20/comments", "issues/20"])

    def test_resume_does_not_recreate_or_overwrite_existing_release(self):
        data = plan()
        api = FakeAPI({"releases/tags/logger%2Fv0.1.1": {"id": 1, "draft": False, "html_url": "https://example.com/release", "name": "logger/v0.1.1", "body": data["modules"][0]["notes"], "prerelease": False}})
        with tempfile.TemporaryDirectory() as temporary, patch.object(release, "ROOT", Path(temporary)), self.context(temporary, data), patch.object(release, "remote_tag", return_value=B), patch.object(release, "verify_consumer", return_value={"sum": "h1:verified"}):
            release.write_json(release.plan_path("logger-update"), data)
            release.publish(self.args(True), api)
        self.assertEqual([path for path, values in api.calls if values.get("method", "GET") != "GET"], ["issues/20/comments", "issues/20"])

    def test_latest_check_result_must_pass(self):
        checks = [{"name": "Build documentation", "app": {"slug": "github-actions"}, "id": number, "status": "completed", "conclusion": conclusion} for number, conclusion in [(1, "success"), (2, "failure")]]
        api = FakeAPI({f"commits/{B}/check-runs?per_page=100&page=1": {"check_runs": checks}})
        with self.assertRaisesRegex(ValueError, "has not passed"):
            release.check_runs(api, B, ["Build documentation"])

    def test_draft_lookup_uses_authenticated_listing_and_rejects_duplicates(self):
        draft = {"tag_name": "logger/v0.1.1", "draft": True}
        api = FakeAPI({"releases": [{"tag_name": "metrics/v0.1.0", "draft": True}, draft]})
        self.assertEqual(release.release_for_tag(api, "logger/v0.1.1"), draft)
        self.assertEqual([path for path, _ in api.calls], ["releases/tags/logger%2Fv0.1.1", "releases"])
        api.responses["releases"].append(draft.copy())
        with self.assertRaisesRegex(ValueError, "Multiple Releases"):
            release.release_for_tag(api, "logger/v0.1.1")

    def test_published_release_lookup_needs_no_draft_listing(self):
        published = {"tag_name": "logger/v0.1.1", "draft": False}
        api = FakeAPI({"releases/tags/logger%2Fv0.1.1": published})
        self.assertEqual(release.release_for_tag(api, "logger/v0.1.1"), published)
        self.assertEqual(len(api.calls), 1)

    def test_annotated_tags_are_peeled(self):
        api = FakeAPI({"git/ref/tags/logger/v0.1.0": {"object": {"type": "tag", "sha": A}}, f"git/tags/{A}": {"object": {"type": "commit", "sha": B}}})
        self.assertEqual(release.remote_tag(api, "logger/v0.1.0"), B)

    def test_conflicting_tags_and_notes_fail_instead_of_overwriting(self):
        selected = item()
        matching = {"name": "logger/v0.1.1", "body": selected["notes"], "prerelease": False}
        release.check_existing("logger", selected, B, B, matching)
        release.check_existing("logger", selected, B, B, {**matching, "body": selected["notes"] + "\n"})
        for target, existing in ((A, None), (None, matching), (B, {**matching, "body": "Different notes."})):
            with self.subTest(target=target), self.assertRaises(ValueError):
                release.check_existing("logger", selected, B, target, existing)

    def test_public_consumers_use_fresh_cache_and_checksums(self):
        calls = []
        def command(*args, **kwargs):
            calls.append((args, kwargs))
            if args[:3] == ("go", "mod", "download"):
                return json.dumps({"Sum": "h1:source", "GoModSum": "h1:mod"})
            if args[:3] == ("go", "list", "-m"):
                return '{"Main":true,"Path":"example.com/release-consumer"}\n{"Path":"logger","Version":"v0.1.1"}'
            return ""
        with tempfile.TemporaryDirectory() as temporary, patch.object(release, "run", side_effect=command), patch.dict(os.environ, {"GOWORK": "go.work", "GOPROXY": "file:///synthetic", "GONOSUMDB": "*", "CGO_ENABLED": "1", "GOFLAGS": "-modfile=private"}):
            self.assertTrue(release.verify_consumer("logger", item(), MODULES["logger"], Path(temporary))["consumer_build"])
        for _, values in calls:
            environment = values["env"]
            self.assertEqual(environment["CGO_ENABLED"], "0")
            self.assertEqual(environment["GOWORK"], "off")
            self.assertEqual(environment["GOPROXY"], "https://proxy.golang.org")
            self.assertEqual(environment["GOSUMDB"], "sum.golang.org")
            self.assertEqual(environment["GOENV"], "off")
            self.assertNotIn("GONOSUMDB", environment)
            self.assertNotIn("GOFLAGS", environment)


class PreflightTests(unittest.TestCase):
    def test_authority_history_and_preparation_scope_before_any_writes(self):
        scenarios = ("valid", "dirty", "local-tag", "permission", "unmerged", "wrong-sha", "closes-issue", "source-changed", "library-change", "closed-issue", "check-failed")
        for scenario in scenarios:
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory() as temporary:
                data = plan()
                responses = {
                    "collaborators/maintainer/permission": {"permission": "admin"},
                    "pulls/21": {"merged": True, "base": {"ref": "main", "repo": {"full_name": release.REPOSITORY}}, "merge_commit_sha": B, "head": {"sha": C}, "body": "Refs #20\n"},
                    "issues/20": {"state": "open"},
                    "releases": [],
                }
                if scenario == "permission":
                    responses["collaborators/maintainer/permission"]["permission"] = "read"
                elif scenario == "unmerged":
                    responses["pulls/21"]["merged"] = False
                elif scenario == "wrong-sha":
                    responses["pulls/21"]["merge_commit_sha"] = A
                elif scenario == "closes-issue":
                    responses["pulls/21"]["body"] += "Closes #20\n"
                elif scenario == "closed-issue":
                    responses["issues/20"]["state"] = "closed"
                api = FakeAPI(responses)
                def git(*args):
                    if args == ("rev-parse", "HEAD"):
                        return B
                    if args == ("status", "--porcelain"):
                        return " M logger/logger.go" if scenario == "dirty" else ""
                    if args[:2] == ("tag", "--list"):
                        return "logger/v0.1.1" if scenario == "local-tag" else ""
                    if args == ("rev-parse", "--verify", "refs/tags/logger/v0.1.1^{commit}"):
                        return A
                    if args == ("rev-parse", B + "^1"):
                        return C if scenario == "source-changed" else A
                    if args[0] == "diff":
                        return "releases/logger-update.json\n" + ("logger/logger.go" if scenario == "library-change" else "tools/modules.json")
                    raise AssertionError(args)
                args = PublicationTests().args()
                with patch.object(release, "ROOT", Path(temporary)), patch.object(release, "manifest", return_value=MODULES), patch.object(release, "git", side_effect=git), patch.object(release, "is_ancestor", return_value=True), patch.object(release, "run", return_value=json.dumps({"Module": {"Path": "github.com/" + release.REPOSITORY + "/logger"}, "Require": []})), patch.object(release, "check_runs", side_effect=ValueError("Check failed") if scenario == "check-failed" else None) as checks, patch.object(release, "previous_release", return_value="logger/v0.1.0"), patch.object(release, "remote_tag", return_value=None), patch.dict(os.environ, {"GITHUB_ACTOR": "maintainer"}):
                    release.write_json(release.plan_path("logger-update"), data)
                    if scenario == "valid":
                        self.assertEqual(release.preflight(args, api)[3], ["logger"])
                        self.assertEqual([call.args[1] for call in checks.call_args_list], [C, B])
                    else:
                        with self.assertRaises(ValueError):
                            release.preflight(args, api)
                self.assertTrue(all(values.get("method", "GET") == "GET" for _, values in api.calls))


class WorkflowTests(unittest.TestCase):
    def test_publication_is_manual_and_serialized(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/release.yml").read_text())
        events = workflow.get("on", workflow.get(True))
        self.assertEqual(set(events), {"workflow_dispatch", "workflow_run"})
        self.assertEqual(set(events["workflow_dispatch"]["inputs"]), {"pr", "publish"})
        self.assertEqual(events["workflow_run"]["workflows"], ["Go CI", "Documentation"])
        self.assertIs(events["workflow_dispatch"]["inputs"]["publish"]["default"], False)
        self.assertFalse(workflow["concurrency"]["cancel-in-progress"])
        self.assertEqual(workflow["env"]["CGO_ENABLED"], "0")
        for step in workflow["jobs"]["release"]["steps"]:
            if "run" in step:
                self.assertNotIn("${{", step["run"])
        self.assertNotIn("secrets.", json.dumps(workflow))
        installs = [step for step in workflow["jobs"]["release"]["steps"] if step.get("uses", "").startswith("goreleaser/goreleaser-action@")]
        self.assertEqual(len(installs), 1)
        self.assertEqual(installs[0]["with"], {"distribution": "goreleaser", "version": "v" + release.GORELEASER_VERSION, "install-only": True})


class GoReleaserTests(unittest.TestCase):
    def test_exact_tags_frozen_notes_and_credentials_are_passed_to_cli(self):
        selected = item("tracer/otlp", "v0.1.1-rc.1", "tracer/otlp/v0.1.0")
        with tempfile.TemporaryDirectory() as temporary, patch.object(release, "run", return_value="CLI output") as command, patch.dict(os.environ, {"GH_TOKEN": "inherited", "GITHUB_TOKEN": "inherited", "GORELEASER_KEY": "unused"}):
            output = Path(temporary)
            config = output / "tracer-otlp/goreleaser.json"
            config.parent.mkdir()
            config.write_bytes((ROOT / ".goreleaser.json").read_bytes())
            args = PublicationTests().args()
            release.run_goreleaser("tracer/otlp", selected, args, output, config)
            dry = command.call_args
            self.assertIn("--skip=validate,publish,announce", dry.args)
            self.assertNotIn("GITHUB_TOKEN", dry.kwargs["env"])
            self.assertNotIn("GH_TOKEN", dry.kwargs["env"])
            self.assertNotIn("GORELEASER_KEY", dry.kwargs["env"])
            self.assertEqual(dry.kwargs["env"]["GORELEASER_CURRENT_TAG"], "tracer/otlp/v0.1.1-rc.1")
            self.assertEqual(dry.kwargs["env"]["GORELEASER_PREVIOUS_TAG"], "tracer/otlp/v0.1.0")
            self.assertEqual(dry.args[dry.args.index("--release-notes") + 1], str(output / "tracer-otlp.md"))
            release.run_goreleaser("tracer/otlp", selected, args, output, config, token="explicit", draft=False)
            publish = command.call_args
            self.assertFalse(json.loads((config.parent / "goreleaser-publish.json").read_text())["release"]["draft"])
            self.assertIn("--skip=validate,announce", publish.args)
            self.assertEqual(publish.kwargs["env"]["GITHUB_TOKEN"], "explicit")
            self.assertEqual((config.parent / "publish.log").read_text(), "CLI output\n")

    @unittest.skipUnless(shutil.which("goreleaser"), "Install pinned GoReleaser OSS to run offline CLI coverage.")
    def test_real_cli_preserves_root_nested_and_prerelease_tags_without_publishing(self):
        with tempfile.TemporaryDirectory(prefix="goreleaser-fixture-") as temporary:
            fixture = Path(temporary)
            (fixture / ".goreleaser.json").write_bytes((ROOT / ".goreleaser.json").read_bytes())
            (fixture / ".gitignore").write_text("dist/\n", encoding="utf-8")
            env = {**os.environ, "CGO_ENABLED": "0"}
            for command in (("git", "init"), ("git", "config", "user.name", "Fixture"), ("git", "config", "user.email", "fixture@example.com"), ("git", "remote", "add", "origin", "https://github.com/" + release.REPOSITORY + ".git"), ("git", "add", "."), ("git", "commit", "-m", "fixture")):
                subprocess.run(command, cwd=fixture, env=env, check=True, capture_output=True)
            sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=fixture, text=True).strip()
            args = type("Args", (), {"sha": sha})()
            output = fixture / "dist/releases/fixture"
            with patch.object(release, "ROOT", fixture):
                for directory, version in ((".", "v0.1.0"), ("logger", "v0.1.1"), ("tracer/otlp", "v0.1.0-rc.1")):
                    with self.subTest(directory=directory):
                        selected = item(directory, version, None)
                        subprocess.run(["git", "tag", release.tag_name(directory, version)], cwd=fixture, check=True, capture_output=True)
                        config = release.goreleaser_config(directory, selected, output)
                        self.assertEqual(json.loads(config.read_text())["release"]["prerelease"], str("-" in version).lower())
                        note = output / (config.parent.name + ".md")
                        note.write_text(selected["notes"], encoding="utf-8", newline="\n")
                        release.run("goreleaser", "check", "--config", str(config), cwd=fixture, env=env)
                        release.run_goreleaser(directory, selected, args, output, config)
                        metadata = json.loads((config.parent / "artifacts/metadata.json").read_text())
                        self.assertEqual(metadata["tag"], release.tag_name(directory, version))
                        self.assertEqual(metadata["commit"], sha)
                        artifacts = json.loads((config.parent / "artifacts/artifacts.json").read_text())
                        self.assertTrue(all(artifact["type"] == "Metadata" for artifact in artifacts))
                        self.assertEqual(note.read_text(), selected["notes"])
            self.assertEqual(subprocess.check_output(["git", "status", "--porcelain"], cwd=fixture, text=True), "")

    @unittest.skipUnless(shutil.which("goreleaser"), "Install pinned GoReleaser OSS to run offline CLI coverage.")
    def test_real_cli_draft_and_finalization_against_local_github_fixture(self):
        state, requests = {}, []
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def respond(self, code, body):
                encoded = json.dumps(body).encode()
                self.send_response(code)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)

            def do_GET(self):
                path = unquote(urlsplit(self.path).path).removeprefix("/api/v3")
                if path == "/rate_limit":
                    rate = {"limit": 5000, "remaining": 5000, "reset": 4102444800}
                    self.respond(200, {"resources": {"core": rate}, "rate": rate})
                elif path.endswith("/releases"):
                    self.respond(200, [state["release"]] if state else [])
                elif "/releases/tags/" in path:
                    published = state.get("release")
                    found = published and not published["draft"]
                    self.respond(200 if found else 404, published if found else {"message": "Not Found"})
                elif path == "/repos/" + release.REPOSITORY:
                    self.respond(200, {"full_name": release.REPOSITORY, "permissions": {"push": True}, "archived": False})
                else:
                    requests.append(("unexpected", path))
                    self.respond(404, {"message": "Not Found"})

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                requests.append(("POST", self.path, body))
                state["release"] = {**body, "id": 7, "html_url": "https://example.com/release", "upload_url": "https://example.com/assets{?name,label}"}
                self.respond(201, state["release"])

            def do_PATCH(self):
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                requests.append(("PATCH", self.path, body))
                state["release"].update(body)
                self.respond(200, state["release"])

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with tempfile.TemporaryDirectory(prefix="goreleaser-api-fixture-") as temporary:
                fixture = Path(temporary)
                base = json.loads((ROOT / ".goreleaser.json").read_text())
                base["github_urls"] = {"api": f"http://127.0.0.1:{server.server_port}/", "download": "https://example.com/"}
                (fixture / ".goreleaser.json").write_text(json.dumps(base), encoding="utf-8")
                (fixture / ".gitignore").write_text("dist/\n", encoding="utf-8")
                for command in (("git", "init"), ("git", "config", "user.name", "Fixture"), ("git", "config", "user.email", "fixture@example.com"), ("git", "remote", "add", "origin", "https://github.com/" + release.REPOSITORY + ".git"), ("git", "add", "."), ("git", "commit", "-m", "fixture")):
                    subprocess.run(command, cwd=fixture, check=True, capture_output=True)
                sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=fixture, text=True).strip()
                args = type("Args", (), {"sha": sha})()
                output = fixture / "dist/releases/fixture"
                with patch.object(release, "ROOT", fixture):
                    for version in ("v0.1.0", "v0.1.1-rc.1"):
                        with self.subTest(version=version):
                            state.clear()
                            requests.clear()
                            selected = item("logger", version, None)
                            subprocess.run(["git", "tag", release.tag_name("logger", version)], cwd=fixture, check=True, capture_output=True)
                            config = release.goreleaser_config("logger", selected, output)
                            (output / "logger.md").write_text(selected["notes"], encoding="utf-8", newline="\n")
                            release.run_goreleaser("logger", selected, args, output, config, token="local-fixture-token")
                            self.assertTrue(state["release"]["draft"])
                            self.assertEqual(state["release"]["name"], "logger/" + version)
                            self.assertEqual(state["release"]["prerelease"], "-" in version)
                            self.assertEqual(state["release"]["body"].rstrip("\r\n"), selected["notes"].rstrip("\r\n"))
                            release.check_existing("logger", selected, sha, sha, state["release"])
                            release.run_goreleaser("logger", selected, args, output, config, token="local-fixture-token", draft=False)
                            self.assertFalse(state["release"]["draft"])
                            release.check_existing("logger", selected, sha, sha, state["release"])
                            self.assertEqual(state["release"]["make_latest"], "false")
                            self.assertEqual(state["release"]["target_commitish"], sha)
                            self.assertEqual(sum(request[0] == "POST" for request in requests), 1)
                            self.assertFalse(any(request[0] == "unexpected" for request in requests), requests)
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == "__main__":
    unittest.main(verbosity=2)
