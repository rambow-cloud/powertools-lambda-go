// Package logger provides Powertools-style structured logging for Go Lambda handlers.
package logger

import (
	"io"
	"math/rand/v2"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// Level uses the same thresholds as Powertools for TypeScript.
type Level int

const (
	TraceLevel    Level = 6
	DebugLevel    Level = 8
	InfoLevel     Level = 12
	WarnLevel     Level = 16
	ErrorLevel    Level = 20
	CriticalLevel Level = 24
	SilentLevel   Level = 28
)

func (l Level) String() string {
	switch l {
	case TraceLevel:
		return "TRACE"
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case CriticalLevel:
		return "CRITICAL"
	case SilentLevel:
		return "SILENT"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel accepts case-insensitive Powertools level names.
func ParseLevel(value string) (Level, bool) {
	for _, l := range []Level{TraceLevel, DebugLevel, InfoLevel, WarnLevel, ErrorLevel, CriticalLevel, SilentLevel} {
		if strings.ToUpper(value) == l.String() {
			return l, true
		}
	}
	return InfoLevel, false
}

type Fields map[string]any

// Formatter can replace the JSON document while retaining the configured sink.
// It must be safe for concurrent calls and must not retain or mutate caller data.
type Formatter func(Fields) (any, error)

// Replacer transforms a value before built-in JSON conversions are applied.
type Replacer func(key string, value any) any

type config struct {
	service        string
	level          Level
	alc            Level
	rate           float64
	stdout, stderr io.Writer
	clock          func() time.Time
	random         func() int
	location       *time.Location
	zone           string
	pretty         bool
	logEvent       bool
	persistent     Fields
	formatter      Formatter
	replacer       Replacer
	order          []string
	buffer         BufferOptions
	onError        func(error)
}

// BufferOptions enables buffering when supplied through WithBuffer.
// MaxBytes defaults to 20480, BufferAt defaults to DEBUG, and errors flush by default.
type BufferOptions struct {
	MaxBytes            int
	BufferAt            Level
	DisableFlushOnError bool
}

type Option func(*config)

func WithServiceName(name string) Option {
	return func(c *config) { c.service = commons.ResolveServiceName(name, "service_undefined") }
}
func WithLevel(level Level) Option {
	return func(c *config) {
		if level.String() != "UNKNOWN" {
			c.level = level
		}
	}
}
func WithSampleRate(rate float64) Option {
	return func(c *config) {
		if rate >= 0 && rate <= 1 {
			c.rate = rate
		}
	}
}
func WithOutput(w io.Writer) Option {
	return func(c *config) {
		if w != nil {
			c.stdout, c.stderr = w, w
		}
	}
}
func WithPersistentKeys(keys Fields) Option {
	return func(c *config) { c.persistent = cloneFields(c.persistent); mergeFields(c.persistent, keys) }
}
func WithFormatter(formatter Formatter) Option { return func(c *config) { c.formatter = formatter } }
func WithReplacer(replacer Replacer) Option    { return func(c *config) { c.replacer = replacer } }
func WithRecordOrder(keys ...string) Option {
	return func(c *config) { c.order = append([]string(nil), keys...) }
}
func WithClock(clock func() time.Time) Option {
	return func(c *config) {
		if clock != nil {
			c.clock = clock
		}
	}
}

// WithRandom supplies an integer in [0, 100), matching the reference sampling grid.
// The function must be safe for concurrent calls.
func WithRandom(random func() int) Option {
	return func(c *config) {
		if random != nil {
			c.random = random
		}
	}
}
func WithBuffer(options BufferOptions) Option {
	return func(c *config) {
		if options.MaxBytes <= 0 {
			options.MaxBytes = 20480
		}
		if options.BufferAt.String() == "UNKNOWN" {
			options.BufferAt = DebugLevel
		}
		c.buffer = options
	}
}

// WithErrorHandler receives wrapper logging errors without replacing handler results.
// The callback must not recursively call this logger.
func WithErrorHandler(handler func(error)) Option {
	return func(c *config) {
		if handler != nil {
			c.onError = handler
		}
	}
}

func defaults() config {
	c := config{service: commons.ResolveServiceName("", "service_undefined"), level: InfoLevel, stdout: os.Stdout, stderr: os.Stderr,
		clock: time.Now, random: func() int { return rand.IntN(100) }, location: time.UTC,
		pretty: commons.IsDevMode(), logEvent: commons.BoolEnvOr("POWERTOOLS_LOGGER_LOG_EVENT", false, false),
		onError: func(error) {}, persistent: Fields{}}
	value := os.Getenv("POWERTOOLS_LOG_LEVEL")
	if value == "" {
		value = os.Getenv("LOG_LEVEL")
	}
	// Environment level names are case-sensitive in the pinned implementation.
	if l, ok := ParseLevel(value); ok && value == l.String() {
		c.level = l
	}
	if rate, err := commons.NumberEnv("POWERTOOLS_LOGGER_SAMPLE_RATE", 0); err == nil && rate >= 0 && rate <= 1 {
		c.rate = rate
	}
	value = os.Getenv("AWS_LAMBDA_LOG_LEVEL")
	if value == "FATAL" {
		value = "CRITICAL"
	}
	if l, ok := ParseLevel(value); ok && value == l.String() {
		c.alc = l
	}
	c.zone = os.Getenv("TZ")
	if c.zone != "" && !strings.Contains(c.zone, "UTC") {
		if loc, err := time.LoadLocation(c.zone); err == nil {
			c.location = loc
		}
	}
	return c
}
