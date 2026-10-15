"""Regression checks for the reader-facing guide contract."""

from pathlib import Path
import tempfile
import unittest

from check_guides import check_application_logging, check_guide, check_logging_surface


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

    def test_application_guides_require_powertools_logger(self):
        text = GUIDE.replace("EXAMPLE.md", "KAFKA.md")
        (self.root / "docs/reference/KAFKA.md").write_text("# Kafka notes\n", encoding="utf-8")
        failures = check_guide(text, "KAFKA", self.root)
        self.assertTrue(any("Powertools Logger" in failure for failure in failures))
        text = text.replace('fmt.Println("Hello")', 'appLog := logger.New()\nappLog.Info("Hello", logger.Fields{})')
        self.assertEqual([], check_guide(text, "KAFKA", self.root))

    def test_logger_guides_preserve_protocol_serialization(self):
        text = GUIDE.replace("EXAMPLE.md", "KAFKA_BINARY.md").replace(
            'fmt.Println("Hello")',
            'appLog := logger.New()\nencoded, err := protojson.Marshal(value)\nappLog.Info("Hello", logger.Fields{"value": jsontext.Value(encoded)})',
        )
        (self.root / "docs/reference/KAFKA_BINARY.md").write_text("# Binary notes\n", encoding="utf-8")
        self.assertEqual([], check_guide(text, "KAFKA_BINARY", self.root))
        for manual in ('json.Marshal(value)', 'fmt.Println(value)'):
            with self.subTest(manual=manual):
                failures = check_guide(text.replace('protojson.Marshal(value)', manual), "KAFKA_BINARY", self.root)
                self.assertTrue(any("manually marshaling" in failure for failure in failures))

    def test_standard_logging_imports_are_rejected_with_any_alias(self):
        for code in ('import "log"', 'import stdlog "log"', 'import (\n applicationlog "log/slog"\n)', 'log.Fatal(err)', 'stdlog.Printf("failed: %v", err)'):
            with self.subTest(code=code):
                self.assertTrue(check_application_logging(code))

    def test_application_printing_is_rejected_including_fmt_aliases(self):
        for code in ('fmt.Println(value)', 'fmt.Fprintf(writer, "%s", value)', 'import output "fmt"\noutput.Print(value)'):
            with self.subTest(code=code):
                self.assertTrue(check_application_logging(code))

    def test_wire_serialization_and_error_formatting_remain_allowed(self):
        code = 'import "fmt"\nerr := fmt.Errorf("invalid %s", name)\nbody := fmt.Sprintf("Hello %s", name)\njson.Marshal(body)\nprotojson.Marshal(message)\nwriter.Write(body)'
        self.assertEqual([], check_application_logging(code))

    def test_logging_scan_covers_secondary_guides_references_readme_and_examples(self):
        pages = ('docs/SECONDARY.md', 'docs/reference/NOTES.md', 'README.md', 'examples/new/main.go')
        for name in pages:
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('```go\nfmt.Println(value)\n```' if name.endswith('.md') else 'package main\nimport "log"', encoding='utf-8')
        failures = check_logging_surface(self.root)
        for name in pages:
            self.assertTrue(any(str(Path(name)) in failure for failure in failures), name)


if __name__ == "__main__":
    unittest.main()
