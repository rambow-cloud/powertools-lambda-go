"""Regression checks for the reader-facing guide contract."""

from pathlib import Path
import tempfile
import unittest

from check_guides import check_guide


GUIDE = '''# Example

**Input**

```text
Hello
```

```go
fmt.Println("Hello")
```

**Output**

```text
Hello
```

## Things to know

The result is ordinary stdout.

## Next steps

[API notes](reference/EXAMPLE.md)
'''


class GuideTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        notes = self.root / "docs/reference/EXAMPLE.md"
        notes.parent.mkdir(parents=True)
        notes.write_text("# API notes\n", encoding="utf-8")

    def check(self, text):
        return check_guide(text, "EXAMPLE", self.root)

    def test_short_code_and_visible_result_pass(self):
        self.assertEqual([], self.check(GUIDE))

    def test_prose_is_not_an_output_example(self):
        failures = self.check(GUIDE.replace("```text\nHello\n```", "It prints Hello."))
        self.assertTrue(any("fenced output" in failure for failure in failures))

    def test_result_must_follow_the_output_label(self):
        self.assertTrue(any("fenced output" in failure for failure in self.check(GUIDE.replace("**Output**", "Expected result"))))

    def test_json_output_must_be_valid(self):
        for invalid in ('{"message":', 'NaN'):
            with self.subTest(invalid=invalid):
                text = GUIDE.replace("```text\nHello", "```json\n" + invalid)
                self.assertTrue(any("invalid JSON" in failure for failure in self.check(text)))

    def test_pinned_versions_belong_in_maintainer_notes(self):
        self.assertTrue(any("upstream versions" in failure for failure in self.check(GUIDE + "\nTypeScript v9.8.7\n")))

    def test_reference_must_exist(self):
        (self.root / "docs/reference/EXAMPLE.md").unlink()
        self.assertTrue(any("reference" in failure for failure in self.check(GUIDE)))

    def test_missing_included_source_is_rejected(self):
        self.assertTrue(any("included example" in failure for failure in self.check(GUIDE + '\n--8<-- "missing.go"\n')))


if __name__ == "__main__":
    unittest.main()
