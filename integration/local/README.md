# Local Docker integration runner

See the maintained [local Docker integration guide](../../docs/LOCAL_INTEGRATION.md) for prerequisites, runner commands, verification scope, and cleanup behavior. Run its commands from the repository root.

For stateful DynamoDB adapter acceptance, run `uv run --no-project python
integration/local/dynamodb_run.py`. It uses the official pinned DynamoDB Local
image and checks conditional claims, replay, recovery, tenant isolation and
Parameters reads/pagination. `--jar path/to/DynamoDBLocal.jar` runs the same suite
with an existing official Java distribution. Reports under
`dist/service-acceptance/` include backend identity, test results and cleanup.
Run `integration/local/test_dynamodb_run.py` for offline report-safeguard tests.
