package main

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

type loggerProbeString string

func (loggerProbeString) MarshalJSON() ([]byte, error) { return []byte(`"custom value"`), nil }

type loggerProbeErrorString string

var loggerProbeMarshalError = errors.New("probe marshaler failed")

func (loggerProbeErrorString) MarshalJSON() ([]byte, error) { return nil, loggerProbeMarshalError }

type loggerProbeText string

func (loggerProbeText) MarshalText() ([]byte, error) { return []byte("custom text"), nil }

type loggerProbeErrorText string

var loggerProbeTextError = errors.New("probe text marshaler failed")

func (loggerProbeErrorText) MarshalText() ([]byte, error) { return nil, loggerProbeTextError }

// loggerParityProbe exercises the public Logger APIs with actual Lambda context
// and invocation scope. Private output keeps probe records out of business logs.
func loggerParityProbe(ctx context.Context) (map[string]any, error) {
	var output bytes.Buffer
	read := func() ([]map[string]any, error) {
		var documents []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
			if line == "" {
				continue
			}
			var document map[string]any
			if err := json.Unmarshal([]byte(line), &document); err != nil {
				return nil, err
			}
			documents = append(documents, document)
		}
		output.Reset()
		return documents, nil
	}
	root := logger.New(logger.WithOutput(&output), logger.WithServiceName("logger-probe"),
		logger.WithBuffer(logger.BufferOptions{}), logger.WithPersistentKeys(logger.Fields{"shared": "parent"}))
	parent := root.WithContext(ctx)
	parent.AppendKeys(logger.Fields{"shared": "temporary", "request": "original"})
	child := parent.Child(logger.WithPersistentKeys(logger.Fields{"shared": "child"}))
	result := map[string]any{"child_persistent": child.PersistentKeys()}
	if err := child.Info("before"); err != nil {
		return nil, err
	}
	child.ResetKeys()
	if err := child.Info("after"); err != nil {
		return nil, err
	}
	if err := parent.Info("parent"); err != nil {
		return nil, err
	}
	if err := child.Info("", logger.Fields{"empty": "", "null_value": nil, "zero": 0, "flag": false,
		"nested": logger.Fields{"empty": "", "null_value": nil}}); err != nil {
		return nil, err
	}
	documents, err := read()
	if err != nil {
		return nil, err
	}
	result["child_records"] = documents
	contextFor := func(ctx context.Context, traceHex string) context.Context {
		headers := http.Header{}
		headers.Set("traceparent", "00-"+traceHex+"-1234567890123456-00")
		return tracer.ExtractHTTPContext(ctx, headers)
	}
	first := root.WithContext(contextFor(context.Background(), "12345678123456789012345678901234"))
	second := root.WithContext(contextFor(context.Background(), "87654321123456789012345678901234"))
	if err := first.Debug("old trace"); err != nil {
		return nil, err
	}
	second.ClearBuffer()
	if err := second.FlushBuffer(); err != nil {
		return nil, err
	}
	result["wrong_trace_no_output"] = output.Len() == 0
	if err := second.Debug("new trace"); err != nil {
		return nil, err
	}
	if err := second.FlushBuffer(); err != nil {
		return nil, err
	}
	documents, err = read()
	if err != nil {
		return nil, err
	}
	result["trace_records"] = documents
	tiny := logger.New(logger.WithOutput(&output), logger.WithServiceName("logger-probe"),
		logger.WithBuffer(logger.BufferOptions{MaxBytes: 10})).WithContext(ctx)
	if err := tiny.Debug("oversize"); err != nil {
		return nil, err
	}
	documents, err = read()
	if err != nil {
		return nil, fmt.Errorf("Logger overflow records: %w", err)
	}
	result["overflow_records"] = documents

	cleanupRoot := logger.New(logger.WithOutput(&output), logger.WithBuffer(logger.BufferOptions{}))
	business := errors.New("probe handler failed")
	handler := logger.WrapHandler(cleanupRoot, func(ctx context.Context, value int) (int, error) {
		// Bind once before tracing starts, then attach an unsampled OTel context.
		bound := cleanupRoot.WithContext(ctx)
		traced := contextFor(ctx, "12345678123456789012345678901234")
		if err := bound.WithContext(traced).Debug("handler trace detail"); err != nil {
			return 0, err
		}
		return value, business
	}, logger.HandlerOptions{FlushBufferOnError: true})
	// Use an independent wrapper scope so cleanup occurs before this probe returns.
	value, handlerErr := handler(context.Background(), 7)
	result["handler_result_preserved"] = value == 7 && handlerErr == business
	documents, err = read()
	if err != nil {
		return nil, err
	}
	result["handler_trace_records"] = documents

	if err := parent.Info("custom string", logger.Fields{"payload": loggerProbeString("")}); err != nil {
		return nil, err
	}
	documents, err = read()
	if err != nil {
		return nil, err
	}
	result["marshaler_records"] = documents
	marshalErr := parent.Info("custom string error", logger.Fields{"payload": loggerProbeErrorString("")})
	result["marshaler_error_preserved"] = errors.Is(marshalErr, loggerProbeMarshalError) && output.Len() == 0
	if err := parent.Info("custom text", logger.Fields{"payload": loggerProbeText("")}); err != nil {
		return nil, err
	}
	documents, err = read()
	if err != nil {
		return nil, err
	}
	result["text_marshaler_records"] = documents
	textErr := parent.Info("custom text error", logger.Fields{"payload": loggerProbeErrorText("")})
	result["text_marshaler_error_preserved"] = errors.Is(textErr, loggerProbeTextError) && output.Len() == 0
	return result, nil
}
