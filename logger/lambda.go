package logger

import (
	"context"
	"fmt"
	"sync"

	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
)

type scopeKey struct{}
type scope struct {
	mu      sync.Mutex
	loggers map[*state]*Logger
	closed  bool
}

func (s *scope) get(l *Logger) (*state, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if bound, ok := s.loggers[l.base]; ok {
		// Retain the most recent binding, including a span started in the handler.
		bound.ctx = l.ctx
		return bound.current, false
	}
	l.base.mu.Lock()
	state := &state{persistent: cloneFields(l.base.persistent), temporary: cloneFields(l.base.temporary), level: l.base.level, closed: s.closed}
	first := !l.base.invoked
	l.base.invoked = true
	l.base.mu.Unlock()
	var sampled bool
	if !first && l.cfg.rate > 0 {
		state.level, sampled = l.cfg.sampleLevel(l.cfg.level)
	}
	bound := *l
	bound.current = state
	s.loggers[l.base] = &bound
	return state, sampled
}

func (s *scope) members() []*Logger {
	s.mu.Lock()
	defer s.mu.Unlock()
	loggers := make([]*Logger, 0, len(s.loggers))
	for _, l := range s.loggers {
		copy := *l
		loggers = append(loggers, &copy)
	}
	return loggers
}

func (s *scope) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	for _, l := range s.members() {
		l.current.mu.Lock()
		l.current.closed = true
		l.current.buffer = nil
		l.current.bytes = 0
		l.current.evicted = false
		l.current.bufferTrace = ""
		l.current.mu.Unlock()
	}
}

func (s *scope) finish(flush bool, handlerErr error) {
	defer s.close()
	loggers := s.members()
	for _, l := range loggers {
		if flush && handlerErr != nil {
			l.report(l.flushBuffer(true))
			l.report(l.Error("Uncaught error detected, flushing log buffer before exit", handlerErr))
		}
	}
}

// Panic diagnostics are best effort, including callbacks and user formatters.
// Keep each operation isolated so one failed sink cannot suppress the main record.
func panicDiagnostic(l *Logger, action func() error) {
	defer func() { _ = recover() }()
	l.report(action())
}

func (s *scope) logPanic(l *Logger, failure any, flush bool) {
	defer s.close()
	for _, member := range s.members() {
		if member.base == l.base {
			l = member
		}
		if flush || !member.cfg.buffer.DisableFlushOnError {
			panicDiagnostic(member, func() error { return member.flushBuffer(true) })
		}
	}
	panicDiagnostic(l, func() error {
		cause, ok := failure.(error)
		if !ok {
			cause = &panicValueError{value: failure}
		}
		if l.cfg.stackTrace {
			cause = &stackError{err: cause, stack: captureStack("panic")}
		}
		// Buffers have already been handled. Always attempt the terminal record,
		// even if a buffer sink panicked or BufferAt includes ERROR.
		diagnostic := *l
		diagnostic.cfg.buffer = BufferOptions{}
		return diagnostic.Error("Unhandled panic", cause)
	})
}

// HandlerOptions controls wrapper behavior. Per-invocation attributes always reset.
// CorrelationID supports custom extraction without imposing a query dependency.
type HandlerOptions struct {
	// LogEvent records the value received by the wrapper before its handler runs.
	// WrapRawHandler receives raw JSON; WrapHandler may receive a typed projection.
	LogEvent           *bool
	FlushBufferOnError bool
	CorrelationID      func(event any) any
	// CorrelationExtractor accepts compiled query expressions without a dependency
	// on a query engine. CorrelationID takes precedence when both are configured.
	CorrelationExtractor interface{ Search(any) (any, error) }
	CorrelationSource    CorrelationSource
}

// WrapHandler enriches logs and isolates mutable state for each Lambda invocation.
// The handler must use l.WithContext(ctx); a root logger has process-scoped state.
// A panic is observed for cleanup and then rethrown unchanged to the Lambda runtime.
// Event logging serializes T, which may already omit original JSON members.
// Use WrapRawHandler at the runtime entry to log before typed decoding.
func WrapHandler[T, R any](l *Logger, handler func(context.Context, T) (R, error), options ...HandlerOptions) func(context.Context, T) (R, error) {
	var opts HandlerOptions
	if len(options) > 0 {
		opts = options[0]
	}
	return func(ctx context.Context, event T) (result R, err error) {
		ctx = invocation.Ensure(ctx)
		s, exists := ctx.Value(scopeKey{}).(*scope)
		if !exists {
			s = &scope{loggers: map[*state]*Logger{}}
			ctx = context.WithValue(ctx, scopeKey{}, s)
		}
		bound := l.WithContext(ctx)
		defer func() {
			failure := recover()
			if failure != nil {
				// This deferred rethrow wins over any secondary diagnostic failure.
				defer func() { panic(failure) }()
			}
			if !exists {
				if failure != nil && l.cfg.panicLogging {
					s.logPanic(bound, failure, opts.FlushBufferOnError)
				} else {
					cleanupErr := err
					if failure != nil {
						cleanupErr = fmt.Errorf("handler panic: %v", failure)
					}
					s.finish(opts.FlushBufferOnError, cleanupErr)
				}
			}
		}()
		if opts.CorrelationID != nil {
			bound.SetCorrelationID(opts.CorrelationID(event))
		} else if opts.CorrelationExtractor != nil {
			value, extractErr := opts.CorrelationExtractor.Search(event)
			bound.report(extractErr)
			if extractErr == nil && value != nil {
				bound.SetCorrelationID(value)
			}
		} else if opts.CorrelationSource != "" {
			value, extractErr := ExtractCorrelationID(opts.CorrelationSource, event)
			bound.report(extractErr)
			if extractErr == nil && value != nil {
				bound.SetCorrelationID(value)
			}
		}
		logEvent := l.cfg.logEvent
		if opts.LogEvent != nil {
			logEvent = *opts.LogEvent
		}
		if logEvent {
			bound.report(bound.Info("Lambda invocation event", Fields{"event": event}))
		}
		return handler(ctx, event)
	}
}
