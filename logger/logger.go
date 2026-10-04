package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
	"go.opentelemetry.io/otel/trace"
)

// ErrInvocationClosed prevents detached work from writing into a completed invocation.
var ErrInvocationClosed = errors.New("logger invocation is closed")

type output struct {
	mu             *sync.Mutex
	stdout, stderr io.Writer
}

func (o *output) write(level Level, data []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	w := o.stdout
	if level >= WarnLevel {
		w = o.stderr
	}
	line := append(append([]byte(nil), data...), '\n')
	n, err := w.Write(line)
	if err == nil && n != len(line) {
		err = io.ErrShortWrite
	}
	return err
}

type entry struct {
	level Level
	data  []byte
	sink  *output
}
type state struct {
	mu                    sync.Mutex
	persistent, temporary Fields
	level                 Level
	buffer                []entry
	bufferTrace           string
	bytes                 int
	evicted               bool
	closed                bool
	invoked               bool
}

// Logger is safe to share across goroutines. Treat values passed as attributes as
// immutable until the call completes; application-owned nested values are not locked.
// Use WithContext inside a wrapped handler to bind invocation-local mutable state.
type Logger struct {
	cfg     config
	sink    *output
	base    *state
	current *state
	ctx     context.Context
}

// New creates a logger. Invalid environment values fall back to defaults.
func New(options ...Option) *Logger {
	c := defaults()
	for _, option := range options {
		option(&c)
	}
	if c.alc != 0 {
		c.level = c.alc
	}
	s := &state{persistent: Fields{}, temporary: Fields{}, level: c.level}
	// Initial keys are filtered without retaining the caller's map.
	for k, v := range c.persistent {
		if !reserved(k) {
			s.persistent[k] = v
		}
	}
	l := &Logger{cfg: c, sink: &output{mu: &sync.Mutex{}, stdout: c.stdout, stderr: c.stderr}, base: s, current: s, ctx: context.Background()}
	if c.zone != "" && !strings.Contains(c.zone, "UTC") {
		if _, err := time.LoadLocation(c.zone); err != nil {
			_ = l.Warn(fmt.Sprintf("Invalid or unresolvable time zone: %q - falling back to UTC.", c.zone))
		}
	}
	l.sampleInitialLevel()
	return l
}

// WithContext binds the logger to the current invocation without mutating the root.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	copy := *l
	copy.ctx = ctx
	if scope, ok := ctx.Value(scopeKey{}).(*scope); ok {
		var sampled bool
		copy.current, sampled = scope.get(&copy)
		if sampled {
			copy.report(copy.Debug(samplingMessage))
		}
	}
	return &copy
}

// Child creates an independent logger with separate snapshots of the parent's
// persistent and temporary attributes, retaining temporary-field precedence.
// Child loggers share the output lock and participate in the same wrapper cleanup.
func (l *Logger) Child(options ...Option) *Logger {
	l.current.mu.Lock()
	c := l.cfg
	c.persistent = cloneFields(l.current.persistent)
	temporary := cloneFields(l.current.temporary)
	c.level = l.current.level
	l.current.mu.Unlock()
	for _, option := range options {
		option(&c)
	}
	if c.alc != 0 {
		c.level = c.alc
	}
	s := &state{persistent: cloneFields(c.persistent), temporary: Fields{}, level: c.level}
	child := &Logger{cfg: c, base: s, current: s, sink: &output{mu: l.sink.mu, stdout: c.stdout, stderr: c.stderr}, ctx: l.ctx}
	child.sampleInitialLevel()
	mergeFields(s.temporary, temporary)
	return child.WithContext(l.ctx)
}

func (l *Logger) GetLevel() Level {
	l.current.mu.Lock()
	defer l.current.mu.Unlock()
	return l.current.level
}

// GetLevelName returns the current invocation's effective log level name.
func (l *Logger) GetLevelName() string { return l.GetLevel().String() }

// GetLogEvent returns the constructor's event logging setting.
func (l *Logger) GetLogEvent() bool { return l.cfg.logEvent }

// GetCorrelationID returns the current temporary correlation ID, as in the reference.
// Treat returned application-owned values as immutable.
func (l *Logger) GetCorrelationID() any {
	l.current.mu.Lock()
	defer l.current.mu.Unlock()
	return l.current.temporary["correlation_id"]
}

func (l *Logger) SetLevel(level Level) {
	if level.String() == "UNKNOWN" {
		return
	}
	if l.cfg.alc != 0 {
		level = l.cfg.alc
	}
	l.current.mu.Lock()
	l.current.level = level
	l.current.mu.Unlock()
}

func (l *Logger) AppendKeys(keys Fields) {
	l.warnReserved(keys)
	l.current.mu.Lock()
	defer l.current.mu.Unlock()
	mergeFields(l.current.temporary, keys)
}

func (l *Logger) AppendPersistentKeys(keys Fields) {
	l.warnReserved(keys)
	l.current.mu.Lock()
	defer l.current.mu.Unlock()
	mergeFields(l.current.persistent, keys)
}

func (l *Logger) RemoveKeys(keys ...string) {
	l.current.mu.Lock()
	defer l.current.mu.Unlock()
	for _, key := range keys {
		delete(l.current.temporary, key)
	}
}

func (l *Logger) RemovePersistentKeys(keys ...string) {
	l.current.mu.Lock()
	defer l.current.mu.Unlock()
	for _, key := range keys {
		delete(l.current.persistent, key)
	}
}

func (l *Logger) ResetKeys() {
	l.current.mu.Lock()
	l.current.temporary = Fields{}
	l.current.mu.Unlock()
}
func (l *Logger) PersistentKeys() Fields {
	l.current.mu.Lock()
	defer l.current.mu.Unlock()
	return cloneFields(l.current.persistent)
}
func (l *Logger) SetCorrelationID(value any) { l.AppendKeys(Fields{"correlation_id": value}) }

func (l *Logger) Trace(message string, extra ...any) error {
	return l.Log(TraceLevel, message, extra...)
}
func (l *Logger) Debug(message string, extra ...any) error {
	return l.Log(DebugLevel, message, extra...)
}
func (l *Logger) Info(message string, extra ...any) error { return l.Log(InfoLevel, message, extra...) }
func (l *Logger) Warn(message string, extra ...any) error { return l.Log(WarnLevel, message, extra...) }
func (l *Logger) Error(message string, extra ...any) error {
	return l.Log(ErrorLevel, message, extra...)
}
func (l *Logger) Critical(message string, extra ...any) error {
	return l.Log(CriticalLevel, message, extra...)
}

func (l *Logger) warnReserved(fields Fields) {
	for key := range fields {
		if reserved(key) {
			l.report(l.Warn(fmt.Sprintf("The key %q is a reserved key and will be dropped.", key)))
		}
	}
}

func (l *Logger) report(err error) {
	if err != nil {
		l.cfg.onError(err)
	}
}

func (l *Logger) record(level Level, message string, extra ...any) Fields {
	fields := Fields{"level": level.String(), "message": message, "timestamp": l.cfg.clock().In(l.cfg.location).Format("2006-01-02T15:04:05.000Z07:00"), "service": l.cfg.service, "sampling_rate": l.cfg.rate}
	if info, ok := invocation.FromContext(l.ctx); ok {
		fields["cold_start"] = info.ColdStart
		fields["function_name"] = os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
		if memory := os.Getenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE"); memory != "" {
			if size, err := strconv.Atoi(memory); err == nil {
				fields["function_memory_size"] = size
			}
		}
	}
	if lc, ok := lambdacontext.FromContext(l.ctx); ok {
		fields["function_request_id"] = lc.AwsRequestID
		fields["function_arn"] = lc.InvokedFunctionArn
		if lc.TenantID != "" {
			fields["tenant_id"] = lc.TenantID
		}
	}
	if id := invocation.TraceID(l.ctx); id != "" {
		fields["xray_trace_id"] = id
	}
	if span := trace.SpanContextFromContext(l.ctx); span.IsValid() {
		fields["trace_id"] = span.TraceID().String()
		fields["span_id"] = span.SpanID().String()
		// Active OTel context takes precedence over stale runtime/process headers.
		fields["xray_trace_id"] = commons.FormatXRayTraceID(span.TraceID().String())
	}
	l.current.mu.Lock()
	for k, v := range l.current.persistent {
		if !reserved(k) {
			fields[k] = v
		}
	}
	for k, v := range l.current.temporary {
		if !reserved(k) {
			fields[k] = v
		}
	}
	l.current.mu.Unlock()
	for _, item := range extra {
		switch value := item.(type) {
		case error:
			fields["error"] = value
		case string:
			fields["extra"] = value
		default:
			if values, ok := asFields(value); ok {
				l.warnReserved(values)
				for k, v := range values {
					if !reserved(k) {
						fields[k] = v
					}
				}
			}
		}
	}
	return fields
}

// Log writes or buffers one log document and returns serialization or output errors.
// Warning and error levels use stderr unless WithOutput supplies a common sink.
func (l *Logger) Log(level Level, message string, extra ...any) error {
	if level == SilentLevel || level.String() == "UNKNOWN" {
		return nil
	}
	l.current.mu.Lock()
	closed := l.current.closed
	threshold := l.current.level
	l.current.mu.Unlock()
	if closed {
		return ErrInvocationClosed
	}
	var flushErr error
	if l.cfg.buffer.MaxBytes > 0 && !l.cfg.buffer.DisableFlushOnError && level >= ErrorLevel {
		flushErr = l.FlushBuffer()
	}
	traceID := l.bufferTraceID()
	buffer := l.cfg.buffer.MaxBytes > 0 && traceID != "" && level <= l.cfg.buffer.BufferAt
	if !buffer && level < threshold {
		return flushErr
	}
	data, err := encode(l.record(level, message, extra...), l.cfg)
	if err != nil {
		return errors.Join(flushErr, err)
	}
	if buffer {
		s := l.current
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return ErrInvocationClosed
		}
		// A new sequential trace discards the previous trace's pending records,
		// including when the new record is too large to buffer.
		if s.bufferTrace != traceID {
			s.buffer = nil
			s.bytes = 0
			s.evicted = false
			s.bufferTrace = traceID
		}
		if len(data) <= l.cfg.buffer.MaxBytes {
			for len(s.buffer) > 0 && s.bytes+len(data) >= l.cfg.buffer.MaxBytes {
				s.bytes -= len(s.buffer[0].data)
				s.buffer[0] = entry{}
				s.buffer = s.buffer[1:]
				s.evicted = true
			}
			s.buffer = append(s.buffer, entry{level, data, l.sink})
			s.bytes += len(data)
			s.mu.Unlock()
			return flushErr
		}
		s.mu.Unlock()
	}
	var warning []byte
	if buffer {
		var e error
		warning, e = encode(l.record(WarnLevel, "Unable to buffer log: Item too big", errors.New("Item too big")), l.cfg)
		flushErr = errors.Join(flushErr, e)
	}
	// Serialize output against invocation closure, including concurrent log writers.
	l.current.mu.Lock()
	defer l.current.mu.Unlock()
	if l.current.closed {
		return ErrInvocationClosed
	}
	if warning != nil {
		flushErr = errors.Join(flushErr, l.sink.write(WarnLevel, warning))
	}
	return errors.Join(flushErr, l.sink.write(level, data))
}

// bufferTraceID uses the same OTel precedence as log record correlation. Span
// changes within a trace retain the buffer, including for unsampled contexts.
func (l *Logger) bufferTraceID() string {
	if span := trace.SpanContextFromContext(l.ctx); span.IsValid() {
		return commons.FormatXRayTraceID(span.TraceID().String())
	}
	return invocation.TraceID(l.ctx)
}

// FlushBuffer emits the active trace's entries without applying the level threshold.
// Without an active trace, or when another trace owns the buffer, it does nothing.
func (l *Logger) FlushBuffer() error {
	return l.flushBuffer(false)
}

// Invocation cleanup owns the isolated state and must flush even when the
// handler attached a new trace after the scope saved this logger's context.
func (l *Logger) flushBuffer(invocationOwned bool) error {
	s := l.current
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrInvocationClosed
	}
	if traceID := l.bufferTraceID(); !invocationOwned && (traceID == "" || s.bufferTrace != traceID) {
		s.mu.Unlock()
		return nil
	}
	var result error
	for _, item := range s.buffer {
		result = errors.Join(result, item.sink.write(item.level, item.data))
	}
	evicted := s.evicted
	s.buffer = nil
	s.bytes = 0
	s.evicted = false
	s.bufferTrace = ""
	s.mu.Unlock()
	if evicted {
		data, err := encode(l.record(WarnLevel, "Some logs are not displayed because they were evicted from the buffer. Increase buffer size to store more logs in the buffer"), l.cfg)
		if err == nil {
			err = l.sink.write(WarnLevel, data)
		}
		result = errors.Join(result, err)
	}
	return result
}

// ClearBuffer discards only the active trace's pending entries.
func (l *Logger) ClearBuffer() {
	l.current.mu.Lock()
	defer l.current.mu.Unlock()
	if traceID := l.bufferTraceID(); traceID == "" || l.current.bufferTrace != traceID {
		return
	}
	l.current.buffer = nil
	l.current.bytes = 0
	l.current.evicted = false
	l.current.bufferTrace = ""
}
