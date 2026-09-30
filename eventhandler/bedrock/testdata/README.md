# Bedrock Agent function reference fixtures

Generated from the actual public Powertools TypeScript v2.35.0 Bedrock resolver
and response builder by `tools/reference/generate-bedrock.mjs`. Full response
envelopes, body strings, diagnostics and calls are compared. Body JSON is not
decoded or reordered for comparison. Special numeric parameter values are tagged
only in recorded handler calls, retaining NaN/infinity/negative-zero distinctions.
Input scenarios are JSON-compatible except explicit tagged return/throw values.
Native Go serialization and malformed Unicode boundaries remain documented gaps.
