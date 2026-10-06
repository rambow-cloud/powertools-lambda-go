// Package batch processes Lambda record batches and reports partial failures.
//
// [NewSQS], [NewSQSFIFO], [NewKinesis] and [NewDynamoDB] create typed processors
// for AWS event records. [New] accepts a custom record [Source]. Configure
// concurrency and failure behavior with [Options].
//
// # Handler composition
//
// [WrapSQS], [WrapKinesis] and [WrapDynamoDB] return Lambda handlers with a
// [Response] containing failed item identifiers. Enable ReportBatchItemFailures
// on the Lambda event source mapping so AWS can apply the partial response.
// [WithParser] composes payload validation without changing record identifiers.
//
// [Processor.Process] returns a [Report] with individual outcomes. When all
// records fail, the report still contains failure identifiers alongside
// [FullBatchFailureError]. FIFO short-circuit rules preserve ordering; consult
// the guide before choosing concurrency or suppressing full-batch errors.
//
// For usage and compatibility details, see the [user guide].
//
// [user guide]: https://github.com/rambow-cloud/powertools-lambda-go/blob/main/docs/BATCH.md
package batch
