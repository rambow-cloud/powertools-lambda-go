"""Synchronize self-contained license notices for every independent Go module."""

import argparse
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SOURCES = ROOT / "third_party/licenses"
COMMIT = "7bcc27b1574493f9452688673658f52b80c53847"


def notices(directory):
    text = """Powertools for Go Lambda - Third-party notices

Original contributions are licensed under the accompanying MIT LICENSE.
Third-party material retains its own copyright and license terms.
This independent project is not an official AWS distribution or endorsement.
External dependencies retain their own licenses; they are not relicensed by
this module. The repository's THIRD_PARTY_NOTICES.md and
docs/THIRD_PARTY_DEPENDENCIES.json document source provenance and direct
dependencies. Those inventories are not a complete binary-distribution SBOM.

Powertools for AWS Lambda (TypeScript)
Reference: v2.35.0, commit """ + COMMIT + """
Source: https://github.com/aws-powertools/powertools-lambda-typescript
License: MIT-0
Scope: compatibility-oriented Go implementations, reference constants,
schemas, examples, and generated behavioral fixtures. The implementation
adapts TypeScript APIs to Go and uses OpenTelemetry for maintained tracing;
it does not claim complete behavioral equivalence.
The following upstream notice and permission text are preserved verbatim.

"""
    for name in ("powertools-typescript-NOTICE", "powertools-typescript-LICENSE"):
        text += (SOURCES / name).read_text(encoding="utf-8").rstrip() + "\n\n"
    if directory in {".", "commons/regex", "tools"}:
        text += """Unicode Character Database 16.0.0
Sources:
https://www.unicode.org/Public/16.0.0/ucd/PropertyAliases.txt
https://www.unicode.org/Public/16.0.0/ucd/PropertyValueAliases.txt
License: Unicode-3.0
Copyright 2024 Unicode, Inc. (the source data headers remain intact).
Scope: tools/reference/unicode source files, commons/regex/unicode_properties.json
and commons/regex/legacy_fold.json. Range and case-fold tables were generated
using Node.js v22.21.1 with Unicode 16.0. The data is not relicensed under MIT.
The following official Unicode license was retrieved on 2026-09-26.

"""
        text += (SOURCES / "Unicode-3.0.txt").read_text(encoding="utf-8").rstrip() + "\n\n"
    if directory in {".", "integration"}:
        text += """OpenTelemetry Lambda collector
Reference: layer-collector/0.23.0
Source: https://github.com/open-telemetry/opentelemetry-lambda
License: Apache-2.0
Scope: integration/collector_components.go.txt and the collector build workflow.
The integration replaces the component factory with an OTLP receiver and an
AWS X-Ray exporter. Downloaded collector sources and binaries are local build
artifacts, not vendored project source. Preserve upstream and dependency
licenses when distributing built collector layers.
The following pinned upstream license is preserved.

"""
        text += (SOURCES / "opentelemetry-lambda-LICENSE").read_text(encoding="utf-8").rstrip() + "\n"
    return text.rstrip() + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="Verify without changing files")
    args = parser.parse_args()
    modules = json.loads((ROOT / "tools/modules.json").read_text(encoding="utf-8"))["modules"]
    license_text = (ROOT / "LICENSE").read_text(encoding="utf-8")
    mismatches = []
    for module in modules:
        directory = module["directory"]
        for filename, expected in (("LICENSE", license_text), ("NOTICE", notices(directory))):
            path = ROOT / directory / filename
            if args.check:
                if not path.is_file() or path.read_text(encoding="utf-8") != expected:
                    mismatches.append(str(path.relative_to(ROOT)))
            elif not path.is_file() or path.read_text(encoding="utf-8") != expected:
                path.write_text(expected, encoding="utf-8", newline="\n")
    if mismatches:
        raise SystemExit("License files need synchronization: " + ", ".join(mismatches))
    print(f"{'Verified' if args.check else 'Synchronized'} LICENSE and NOTICE in {len(modules)} modules.")


if __name__ == "__main__":
    main()
