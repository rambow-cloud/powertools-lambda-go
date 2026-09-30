package metrics

import (
	"context"
	"os"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
)

type contextKey struct{ base *state }

func (m *Metrics) WithContext(ctx context.Context) *Metrics {
	copy := *m
	if current, ok := ctx.Value(contextKey{m.base}).(*state); ok {
		copy.current = current
	}
	return &copy
}

type HandlerOptions struct {
	CaptureColdStart bool
	// ThrowOnEmptyMetrics enables the scoped policy. False does not disable an inherited policy.
	ThrowOnEmptyMetrics bool
	// DefaultDimensions merges invocation defaults before cold-start capture.
	DefaultDimensions Dimensions
	// PropagateErrors lets publication errors replace the handler result, error or panic,
	// matching the reference finally/after behavior. The default preserves business outcomes.
	PropagateErrors bool
}

// StartScope creates independent metric storage and binds it to the returned context.
// It inherits the active scope's default dimensions, but not pending metrics or metadata.
// Call finish after all scoped work: it flushes once, rejects late writes, and returns
// the same flush error on repeated calls. The writer and configuration remain shared.
func (m *Metrics) StartScope(ctx context.Context) (context.Context, func() error) {
	ctx, bound := m.startScope(ctx)
	var once sync.Once
	var err error
	return ctx, func() error {
		once.Do(func() {
			err = bound.flushScope(true)
		})
		return err
	}
}

func (m *Metrics) startScope(ctx context.Context) (context.Context, *Metrics) {
	ctx = invocation.Ensure(ctx)
	parent := m.WithContext(ctx).current
	parent.mu.Lock()
	current := newState(parent.defaults)
	current.requireMetrics = parent.requireMetrics
	current.functionName = parent.functionName
	info, _ := invocation.FromContext(ctx)
	current.invocationCold = &info.ColdStart
	parent.mu.Unlock()
	ctx = context.WithValue(ctx, contextKey{m.base}, current)
	return ctx, m.WithContext(ctx)
}

func (m *Metrics) closeScope() {
	s := m.current
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
	s.closed.Store(true)
}

func (m *Metrics) flushScope(final bool) error {
	s := m.current
	s.mu.Lock()
	defer m.unlock(s)
	if final {
		defer s.closed.Store(true)
	}
	return m.flushLocked()
}

// WrapHandler flushes on success, error, and panic and rejects late request writes.
// Flush errors go to WithErrorHandler. Business outcomes are preserved unless
// HandlerOptions.PropagateErrors selects reference publication-error precedence.
// Use WithContext inside the handler. Root-level pending metrics are not copied into requests.
func WrapHandler[T, R any](m *Metrics, handler func(context.Context, T) (R, error), options ...HandlerOptions) func(context.Context, T) (R, error) {
	return WrapHandlers([]*Metrics{m}, handler, options...)
}

// WrapHandlers applies the reference middleware's ordered multi-instance lifecycle.
// It snapshots the target list and options, isolates request storage, and stops
// publication at the first error while closing every scope that it owns.
// Duplicate targets share storage and retain their repeated setup/publication calls.
// Existing outer scopes remain owned by their original wrapper.
func WrapHandlers[T, R any](targets []*Metrics, handler func(context.Context, T) (R, error), options ...HandlerOptions) func(context.Context, T) (R, error) {
	targets = append([]*Metrics(nil), targets...)
	for _, target := range targets {
		if target == nil {
			panic("metrics: nil wrapper target")
		}
	}
	var opts HandlerOptions
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.DefaultDimensions != nil {
		opts.DefaultDimensions = copyDimensions(opts.DefaultDimensions)
	}
	return func(ctx context.Context, event T) (result R, err error) {
		ctx = invocation.Ensure(ctx)
		owned := make(map[*state]*Metrics, len(targets))
		entries := make([]*Metrics, 0, len(targets))
		last := make(map[*state]int, len(targets))
		closeAll := func() {
			for _, bound := range owned {
				bound.closeScope()
			}
		}
		prepared := false
		defer func() {
			failure := recover()
			// Close all scopes even when publication or a user callback panics.
			defer closeAll()
			if prepared {
				for i, bound := range entries {
					if flushErr := bound.flushScope(last[bound.base] == i); flushErr != nil {
						closeAll()
						bound.cfg.onError(flushErr)
						if opts.PropagateErrors {
							var zero R
							result, err, failure = zero, flushErr, nil
						}
						break
					}
				}
			}
			if failure != nil {
				panic(failure)
			}
		}()
		for _, target := range targets {
			bound := owned[target.base]
			if bound == nil {
				if _, exists := ctx.Value(contextKey{target.base}).(*state); exists {
					continue
				}
				ctx, bound = target.startScope(ctx)
				owned[target.base] = bound
			}
			entries = append(entries, bound)
			last[target.base] = len(entries) - 1
			if err := bound.prepareHandler(opts); err != nil {
				bound.cfg.onError(err)
				return result, err
			}
		}
		prepared = true
		return handler(ctx, event)
	}
}

func (m *Metrics) prepareHandler(options HandlerOptions) error {
	if options.ThrowOnEmptyMetrics {
		if err := m.SetThrowOnEmptyMetrics(true); err != nil {
			return err
		}
	}
	if options.DefaultDimensions != nil {
		if err := m.SetDefaultDimensions(options.DefaultDimensions); err != nil {
			return err
		}
	}
	if options.CaptureColdStart {
		return m.CaptureColdStartMetric(os.Getenv("AWS_LAMBDA_FUNCTION_NAME"))
	}
	return nil
}
