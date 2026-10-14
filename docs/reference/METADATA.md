# Metadata: API and behavior notes

Start with the [Metadata guide](../METADATA.md) for examples and results. This page covers detailed contracts and less common options. Contributor provenance and verification scope are recorded in [compatibility](COMPATIBILITY.md) and the linked implementation plans.

## Objects and lifecycle

| Object | Responsibility |
| --- | --- |
| Default helper | One lazily used process client; a nonempty successful result is cached across warm calls |
| `Client` | `metadata.New(Config)` creates an independent client with endpoint, token and optional HTTP client |
| `Options` / `ctx` | Per-call timeout bounded by the caller's cancellation/deadline; default timeout is one second |
| Returned map | Private snapshot; caller changes do not corrupt cached metadata |

Call `ClearMetadataCache()` to clear the default client, or `client.ClearCache()` for an explicit client. Failures and empty responses are not cached. Concurrent fetches on one client are coalesced; waiting callers can cancel, and a pre-clear request cannot refill the cleared cache. Explicit `Endpoint` enables local HTTP access outside Lambda. The client rejects redirects so authentication stays on the configured endpoint.

