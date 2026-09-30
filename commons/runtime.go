package commons

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
)

func InitializationType() string {
	value, _ := StringEnv("AWS_LAMBDA_INITIALIZATION_TYPE", "unknown")
	if value == "on-demand" || value == "provisioned-concurrency" {
		return value
	}
	return "unknown"
}

// Utility maps the reference per-instance cold-start helper. Composed Lambda wrappers
// retain their separate, shared invocation identity in internal/invocation.
type Utility struct {
	initialization string
	used           atomic.Bool
}

func NewUtility() *Utility                       { return &Utility{initialization: InitializationType()} }
func (u *Utility) GetInitializationType() string { return u.initialization }
func (u *Utility) GetColdStart() bool            { return u.initialization == "on-demand" && !u.used.Swap(true) }
func IsValidServiceName(name string) bool        { return TrimSpace(name) != "" }

func ShouldUseInvokeStore() bool {
	value, _ := StringEnv("AWS_LAMBDA_MAX_CONCURRENCY", "")
	return value != ""
}

// TraceHeader prefers request-local data, including an explicitly empty header.
// Concurrent Go invocations must not fall back to a process-wide trace header.
func TraceHeader(ctx context.Context) string {
	if value, ok := ctx.Value("x-amzn-trace-id").(string); ok {
		return value
	}
	if value := os.Getenv("AWS_LAMBDA_MAX_CONCURRENCY"); value != "" && value != "1" {
		return ""
	}
	value, _ := StringEnv("_X_AMZN_TRACE_ID", "")
	return value
}
func XRayTraceData(ctx context.Context) map[string]string {
	header := TraceHeader(ctx)
	if header == "" {
		return nil
	}
	if !strings.Contains(header, "=") {
		return map[string]string{"Root": header}
	}
	result := make(map[string]string)
	for _, field := range strings.Split(header, ";") {
		parts := strings.Split(field, "=")
		if len(parts) > 1 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}
func XRayTraceID(ctx context.Context) string        { return XRayTraceData(ctx)["Root"] }
func IsRequestXRaySampled(ctx context.Context) bool { return XRayTraceData(ctx)["Sampled"] == "1" }

// FormatXRayTraceID converts a nonzero 32-digit trace ID to the X-Ray wire format.
// This formats an identifier without importing either tracing SDK.
func FormatXRayTraceID(id string) string {
	if len(id) != 32 || id == strings.Repeat("0", 32) {
		return ""
	}
	id = strings.ToLower(id)
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return ""
		}
	}
	return "1-" + id[:8] + "-" + id[8:]
}
