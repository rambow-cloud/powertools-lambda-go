"""Offline regression tests for the exact metadata script embedded in Actions."""

import json
from pathlib import Path
import unittest
from unittest.mock import patch

import yaml

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = yaml.safe_load((ROOT / ".github/workflows/contribution.yml").read_text())
RUN = WORKFLOW["jobs"]["policy"]["steps"][0]["run"]
PREFIX = "python3 - <<'PY'\n"
assert RUN.startswith(PREFIX) and RUN.rstrip().endswith("\nPY")
SCRIPT = RUN[len(PREFIX):].rstrip()[:-3]
NAMESPACE = {"__name__": "contribution_policy"}
exec(compile(SCRIPT, "contribution.yml", "exec"), NAMESPACE)
REPOSITORY = "rambow-cloud/powertools-lambda-go"


def body(reference="Closes #123", summary="Fix the issue.", testing="Tests passed."):
    return f"## Issue\n\n{reference}\n\n## Summary\n\n{summary}\n\n## Testing\n\n{testing}\n\n## Release notes\n\n- logger | fix | Correct the reported behavior.\n"


class ContributionPolicyTests(unittest.TestCase):
    def validate(self, text, issue=None, author="contributor"):
        lookups = []

        def fetch(number):
            lookups.append(number)
            return issue if issue is not None else {"number": number, "state": "open"}

        errors = NAMESPACE["validate"]({"body": text, "user": {"login": author}}, REPOSITORY, fetch)
        return errors, lookups

    def test_local_reference_and_sections(self):
        self.assertEqual(self.validate(body()), ([], [123]))

    def test_keywords_and_same_repository_forms(self):
        for keyword in ("Close", "Closes", "Closed", "Fix", "Fixes", "Fixed", "Resolve", "Resolves", "Resolved", "Refs"):
            for reference in ("#123", f"{REPOSITORY}#123", f"https://github.com/{REPOSITORY}/issues/123"):
                with self.subTest(keyword=keyword, reference=reference):
                    self.assertEqual(self.validate(body(f"{keyword} {reference}")), ([], [123]))

    def test_comments_and_fenced_examples_are_not_references(self):
        for reference in ("<!-- Closes #123 -->", "<!--\nCloses #123\n-->", "```text\nCloses #123\n```", "~~~\nCloses #123\n~~~", "`Closes #123`", "<pre>\nCloses #123\n</pre>", "<code>\nCloses #123", "```text\nCloses #123", "````\nCloses #123\n`````", "    Closes #123", "> Closes #123", "<!--\nCloses #123"):
            with self.subTest(reference=reference):
                errors, lookups = self.validate(body(reference))
                self.assertTrue(errors)
                self.assertEqual(lookups, [])

    def test_external_or_invalid_reference(self):
        for reference in ("Closes other/repo#123", "Closes https://github.com/other/repo/issues/123", "Closes https://github.com/" + REPOSITORY + "/pull/123", "Closes #0", "Closes #123abc", "Closes #123/extra", "Closes #123 $(echo injected)"):
            with self.subTest(reference=reference):
                self.assertTrue(self.validate(body(reference))[0])

    def test_pull_request_number_is_rejected(self):
        errors, lookups = self.validate(body(), {"number": 123, "pull_request": {}})
        self.assertEqual(lookups, [123])
        self.assertIn("is a pull request", errors[0])

    def test_owner_and_bot_have_no_exemption(self):
        for author in ("jinxiao", "dependabot[bot]", "contributor"):
            with self.subTest(author=author):
                self.assertTrue(self.validate(body(""), author=author)[0])

    def test_missing_description_and_sections(self):
        for text in (None, "", "Closes #123", body(summary="<!-- add text -->"), body(testing="<!-- add tests -->"), body(testing="- [ ]")):
            with self.subTest(text=text):
                self.assertTrue(self.validate(text)[0])

    def test_testing_accepts_command_blocks_or_explanations(self):
        for testing in ("```sh\nCGO_ENABLED=0 go test ./logger/...\n```\nPassed.", "Not run locally: Go unavailable; waiting for full CI."):
            with self.subTest(testing=testing):
                self.assertEqual(self.validate(body(testing=testing)), ([], [123]))

    def test_multiple_references_and_deduplication(self):
        self.assertEqual(self.validate(body("Closes #124\nRefs #123\nRefs #123")), ([], [123, 124]))

    def test_excessive_references_are_rejected_without_network(self):
        errors, lookups = self.validate(body("\n".join(f"Refs #{n}" for n in range(1, 12))))
        self.assertTrue(errors)
        self.assertEqual(lookups, [])

    def test_lookup_failure_does_not_pass(self):
        def unavailable(number):
            raise RuntimeError("HTTP 404")
        with self.assertRaisesRegex(RuntimeError, "404"):
            NAMESPACE["validate"]({"body": body()}, REPOSITORY, unavailable)

    def test_untrusted_body_is_never_executed(self):
        marker = ROOT / "dist/contribution-injection-must-not-exist"
        self.assertFalse(marker.exists())
        self.assertEqual(self.validate(body(summary=f"$(touch {marker}) ${{{{ github.token }}}}")), ([], [123]))
        self.assertFalse(marker.exists())

    def test_api_error_fails_with_actionable_message(self):
        error = NAMESPACE["HTTPError"]("https://api.github.com", 403, "Forbidden", {}, None)
        environment = {"GITHUB_API_URL": "https://api.github.com", "GITHUB_REPOSITORY": REPOSITORY, "GH_TOKEN": "test-token"}
        with patch.dict(NAMESPACE["os"].environ, environment), patch.dict(NAMESPACE, {"urlopen": lambda *a, **kw: (_ for _ in ()).throw(error)}):
            with self.assertRaisesRegex(RuntimeError, "HTTP 403"):
                NAMESPACE["read_api"]("issues/123")

    def test_workflow_is_read_only_without_checkout(self):
        self.assertTrue(all(value == "read" for value in WORKFLOW["permissions"].values()))
        self.assertTrue(all("uses" not in step for step in WORKFLOW["jobs"]["policy"]["steps"]))
        # PyYAML treats the YAML 1.1 key 'on' as True; GitHub uses YAML 1.2.
        events = WORKFLOW.get("on", WORKFLOW.get(True))
        self.assertEqual(set(events), {"pull_request", "workflow_dispatch"})
        trigger = events["pull_request"]
        self.assertEqual(trigger["branches"], ["main"])
        self.assertEqual(set(trigger["types"]), {"opened", "edited", "synchronize", "reopened", "ready_for_review"})
        self.assertNotIn("paths", trigger)
        self.assertEqual(WORKFLOW["env"]["CGO_ENABLED"], "0")
        self.assertNotIn("${{", RUN)

    def test_release_notes_are_required_and_structured(self):
        for notes in ("", "<!-- - logger | fix | Example. -->", "```\n- logger | fix | Example.\n```", "- logger | unknown | Example.", "None:"):
            with self.subTest(notes=notes):
                self.assertTrue(self.validate(body().split("## Release notes")[0] + "## Release notes\n\n" + notes)[0])
        for notes in ("None: Tests only; no runtime behavior changes.", "- . | fix | Correct shared invocation state.\n- logger | feature | Add configuration."):
            self.assertEqual(self.validate(body().split("## Release notes")[0] + "## Release notes\n\n" + notes), ([], [123]))

    def test_dispatched_policy_checks_are_bound_to_the_actual_pr_head(self):
        environment = {"GITHUB_EVENT_NAME": "workflow_dispatch", "DISPATCH_PR": "123", "GITHUB_SHA": "b" * 40, "GITHUB_REPOSITORY": REPOSITORY, "GITHUB_EVENT_PATH": "event.json"}
        for head_sha, repository in (("b" * 40, REPOSITORY), ("a" * 40, REPOSITORY), ("b" * 40, "external/fork")):
            pr = {"body": body(), "head": {"sha": head_sha, "repo": {"full_name": repository}}}
            def read_api(path):
                return pr if path.startswith("pulls/") else {"number": 123}
            with self.subTest(head=head_sha, repository=repository), patch.dict(NAMESPACE["os"].environ, environment), patch.dict(NAMESPACE, {"read_api": read_api}), patch.object(NAMESPACE["Path"], "read_text", return_value="{}"):
                if head_sha == environment["GITHUB_SHA"] and repository == REPOSITORY:
                    NAMESPACE["main"]()
                else:
                    with self.assertRaisesRegex(ValueError, "exact head SHA"):
                        NAMESPACE["main"]()


class IssueFormTests(unittest.TestCase):
    def test_forms_have_unique_fields_and_required_inputs(self):
        forms = sorted((ROOT / ".github/ISSUE_TEMPLATE").glob("*.yml"))
        self.assertEqual(len(forms), 8)
        for path in forms:
            with self.subTest(path=path.name):
                form = yaml.safe_load(path.read_text())
                if path.name == "config.yml":
                    self.assertIs(form["blank_issues_enabled"], False)
                    continue
                self.assertTrue(form["name"] and form["description"])
                expected_labels = {
                    "bug_report.yml": ["bug"], "feature_request.yml": ["enhancement"],
                    "documentation.yml": ["documentation"], "cicd.yml": ["cicd"],
                    "maintenance.yml": ["maintenance"], "question.yml": ["question"],
                    "release.yml": ["release"],
                }
                self.assertEqual(form.get("labels", []), expected_labels.get(path.name, []))
                self.assertNotIn("assignees", form)
                fields = [field for field in form["body"] if field["type"] != "markdown"]
                ids = [field["id"] for field in fields]
                self.assertEqual(len(ids), len(set(ids)))
                self.assertTrue(any(field.get("validations", {}).get("required") for field in fields))

    def test_form_labels_exist_in_valid_catalog(self):
        labels = json.loads((ROOT / ".github/labels.json").read_text(encoding="utf-8"))
        names = [label["name"] for label in labels]
        self.assertEqual(len(names), len(set(names)))
        for label in labels:
            with self.subTest(label=label["name"]):
                self.assertRegex(label["name"], r"^[a-z][a-z ]+$")
                self.assertRegex(label["color"], r"^[0-9a-f]{6}$")
                self.assertTrue(0 < len(label["description"]) <= 100)
        self.assertEqual(next(label["color"] for label in labels if label["name"] == "bug"), "d73a4a")
        for path in (ROOT / ".github/ISSUE_TEMPLATE").glob("*.yml"):
            form = yaml.safe_load(path.read_text())
            self.assertTrue(set(form.get("labels", [])) <= set(names), path.name)

    def test_unfilled_pr_template_does_not_pass(self):
        text = (ROOT / ".github/PULL_REQUEST_TEMPLATE.md").read_text()
        errors = NAMESPACE["validate"]({"body": text}, REPOSITORY, lambda number: {})
        self.assertEqual(len(errors), 4)


if __name__ == "__main__":
    unittest.main(verbosity=2)
