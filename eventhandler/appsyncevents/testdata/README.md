# AppSync Events reference fixtures

Generate the pinned TypeScript v2.35.0 scenarios with `node generate-appsync-events.mjs` in `tools/reference`. The fixture records real resolver output, calls and diagnostics. Top-level undefined maps to null; property omission remains exact. Large strings use exact UTF-8 byte lengths and SHA-256 digests to keep the fixture bounded. Per-item handler calls and error logs are compared as multisets because Go executes them concurrently. Route diagnostics and response item order remain exact.
