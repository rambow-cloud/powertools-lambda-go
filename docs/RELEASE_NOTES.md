---
description: "Read published Powertools for Go release notes, stable and prerelease versions, component changes and canonical GitHub Release links."
---

# Release notes

All 27 maintained modules share one version. [Browse all GitHub Releases](https://github.com/rambow-cloud/powertools-lambda-go/releases) for full changelogs.

## v1.1.0

2026-10-10 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.1.0)

- **Logger:** Add [`WrapRawHandler`](LOGGER.md#event-logging-and-child-configuration) to log incoming JSON before typed decoding, preserving unknown members and original values.

## v1.0.0

2026-10-07 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.0.0)

- First stable v1 release with the [maintained API compatibility commitment](COMPATIBILITY.md).
- Require Go 1.27 and use JSON v2 directly.

## v1.0.0-rc.1

2026-10-07 · Prerelease · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v1.0.0-rc.1)

- Preview the v1 API with Go 1.27 and JSON v2.

## v0.2.0

2026-10-05 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v0.2.0)

- **Parameters, Parser and JMESPath:** Fix UTF-8 decoding; Parameters and Parser also strip leading BOMs.
- **Signer:** Preserve request bodies during asynchronous uploads.
- Release all maintained modules at one shared version with aligned dependencies.

## v0.1.0

2026-10-04 · [GitHub Release](https://github.com/rambow-cloud/powertools-lambda-go/releases/tag/v0.1.0)

- Initial public release of 27 maintained Lambda utilities.
