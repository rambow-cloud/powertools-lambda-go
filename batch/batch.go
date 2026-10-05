// Package batch processes Lambda record batches and reports partial failures.
package batch

import (
	"context"
	"fmt"
	"sync"
)

// Response is the Lambda ReportBatchItemFailures response for all three sources.
type Response struct {
	BatchItemFailures []ItemFailure `json:"batchItemFailures"`
}
type ItemFailure struct {
	ItemIdentifier string `json:"itemIdentifier"`
}

type Handler[T, R any] func(context.Context, T) (R, error)

// Source defines record identity and, optionally, FIFO message grouping.
// A non-nil GroupID enables FIFO processing. Callbacks must not panic or mutate
// records and must be safe for concurrent invocations.
type Source[T any] struct {
	Identifier func(T) string
	GroupID    func(T) string
	// DynamoDB's reference collector omits absent/empty sequence identifiers.
	OmitEmptyIdentifier bool
}

// Options are fixed at construction and never shared as mutable invocation state.
type Options struct {
	Sequential bool
	// Zero permits all records concurrently, matching the reference default.
	MaxConcurrency           int
	SuppressFullBatchFailure bool
	SkipGroupOnError         bool
}

type RecordResult[T, R any] struct {
	Record T
	Result R
	Err    error
	// Skipped marks a record not delivered to the handler (FIFO or cancellation).
	Skipped bool
}

// Report belongs to one call. Results retain input order; success/failure lists
// retain completion order. Records and handler return values remain caller-owned.
type Report[T, R any] struct {
	Response  Response
	Results   []RecordResult[T, R]
	Successes []T
	Failures  []T
	Errors    []error
}

// Processor is immutable and safe for warm reuse and concurrent invocations.
type Processor[T, R any] struct {
	source  Source[T]
	options Options
}

func New[T, R any](source Source[T], options Options) (*Processor[T, R], error) {
	if source.Identifier == nil {
		return nil, &ConfigurationError{"record identifier function is required"}
	}
	if options.MaxConcurrency < 0 {
		return nil, &ConfigurationError{"MaxConcurrency must not be negative"}
	}
	if options.SkipGroupOnError && source.GroupID == nil {
		return nil, &ConfigurationError{"SkipGroupOnError requires a FIFO source"}
	}
	return &Processor[T, R]{source: source, options: options}, nil
}

// Process waits for all started handlers. Cancellation prevents new handler
// calls and marks unstarted records as failures; handlers must honor context.
// A handler panic becomes a record-level PanicError, like a thrown JS exception.
func (p *Processor[T, R]) Process(ctx context.Context, records []T, handler Handler[T, R]) (Report[T, R], error) {
	report := Report[T, R]{Response: Response{BatchItemFailures: []ItemFailure{}}, Results: make([]RecordResult[T, R], len(records)), Successes: []T{}, Failures: []T{}, Errors: []error{}}
	if handler == nil {
		return report, &ConfigurationError{"record handler is required"}
	}
	var lock sync.Mutex
	finish := func(index int, result R, err error, skipped bool) {
		lock.Lock()
		defer lock.Unlock()
		record := records[index]
		report.Results[index] = RecordResult[T, R]{Record: record, Result: result, Err: err, Skipped: skipped}
		if err == nil {
			report.Successes = append(report.Successes, record)
		} else {
			report.Failures = append(report.Failures, record)
			report.Errors = append(report.Errors, err)
		}
	}
	run := func(index int) error {
		var zero R
		if err := ctx.Err(); err != nil {
			finish(index, zero, err, true)
			return err
		}
		result, err := invoke(ctx, records[index], handler)
		finish(index, result, err, false)
		return err
	}
	if p.source.GroupID != nil {
		failed := false
		groups := map[string]bool{}
		for i, record := range records {
			group := p.source.GroupID(record)
			var skip error
			if failed && !p.options.SkipGroupOnError {
				skip = &FIFOShortCircuitError{}
			} else if group != "" && groups[group] {
				skip = &FIFOGroupShortCircuitError{GroupID: group}
			}
			if skip != nil {
				var zero R
				finish(i, zero, skip, true)
				continue
			}
			if run(i) != nil {
				failed = true
				if group != "" {
					groups[group] = true
				}
			}
		}
	} else if p.options.Sequential || p.options.MaxConcurrency == 1 {
		for i := range records {
			run(i)
		}
	} else {
		workers := len(records)
		if p.options.MaxConcurrency > 0 {
			workers = min(workers, p.options.MaxConcurrency)
		}
		var wg sync.WaitGroup
		jobs := make(chan int)
		for range workers {
			wg.Go(func() {
				for index := range jobs {
					run(index)
				}
			})
		}
		for index := range records {
			jobs <- index
		}
		close(jobs)
		wg.Wait()
	}
	for _, record := range report.Failures {
		id := p.source.Identifier(record)
		if id != "" || !p.source.OmitEmptyIdentifier {
			report.Response.BatchItemFailures = append(report.Response.BatchItemFailures, ItemFailure{ItemIdentifier: id})
		}
	}
	if len(report.Errors) > 0 && len(report.Errors) == len(records) && !p.options.SuppressFullBatchFailure {
		return report, &FullBatchFailureError{RecordErrors: append([]error(nil), report.Errors...)}
	}
	return report, nil
}

func invoke[T, R any](ctx context.Context, record T, handler Handler[T, R]) (result R, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = &PanicError{Value: failure}
		}
	}()
	return handler(ctx, record)
}

type ConfigurationError struct{ Message string }

func (e *ConfigurationError) Error() string { return e.Message }

type FullBatchFailureError struct{ RecordErrors []error }

func (e *FullBatchFailureError) Error() string   { return "all records failed processing" }
func (e *FullBatchFailureError) Unwrap() []error { return e.RecordErrors }

type FIFOShortCircuitError struct{}

func (*FIFOShortCircuitError) Error() string {
	return "a previous record failed; remaining records were not processed to preserve FIFO order"
}

type FIFOGroupShortCircuitError struct{ GroupID string }

func (*FIFOGroupShortCircuitError) Error() string {
	return "a previous record from this message group failed processing"
}

type PanicError struct{ Value any }

func (e *PanicError) Error() string { return fmt.Sprintf("record handler panicked: %v", e.Value) }
func (e *PanicError) Unwrap() error { err, _ := e.Value.(error); return err }

type UnexpectedBatchTypeError struct{}

func (*UnexpectedBatchTypeError) Error() string {
	return "expected a Lambda event with a non-null Records array"
}
