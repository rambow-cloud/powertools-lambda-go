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

func (s *scope) finish(flush bool, handlerErr error) {
	s.mu.Lock()
	s.closed = true
	loggers := make([]*Logger, 0, len(s.loggers))
	for _, l := range s.loggers {
		loggers = append(loggers, l)
	}
	s.mu.Unlock()
	for _, l := range loggers {
		if flush && handlerErr != nil {
			l.report(l.FlushBuffer())
			l.report(l.Error("Uncaught error detected, flushing log buffer before exit", handlerErr))
		}
		l.current.mu.Lock()
		l.current.closed = true
		l.current.buffer = nil
		l.current.bytes = 0
		l.current.mu.Unlock()
	}
}

// HandlerOptions controls wrapper behavior. Per-invocation attributes always reset.
// CorrelationID supports custom extraction without imposing a query dependency.
type HandlerOptions struct {
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
			cleanupErr := err
			if failure != nil {
				cleanupErr = fmt.Errorf("handler panic: %v", failure)
			}
			if !exists {
				s.finish(opts.FlushBufferOnError, cleanupErr)
			}
			if failure != nil {
				panic(failure)
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
