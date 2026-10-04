package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

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
	contextFor := func(traceHex string) context.Context {
		headers := http.Header{}
		headers.Set("traceparent", "00-"+traceHex+"-1234567890123456-00")
		return tracer.ExtractHTTPContext(context.Background(), headers)
	}
	first := root.WithContext(contextFor("12345678123456789012345678901234"))
	second := root.WithContext(contextFor("87654321123456789012345678901234"))
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
	return result, nil
}
