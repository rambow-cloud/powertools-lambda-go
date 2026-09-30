package http

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	nethttp "net/http"
	"strconv"
	"strings"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// CompressionOptions selects gzip (default) or deflate (the zlib format).
// A nil Threshold uses 1024 bytes; compression requires a strictly larger body.
type CompressionOptions struct {
	Encoding  string
	Threshold *float64
}

// Compress preserves the pinned middleware's substring-based encoding selection,
// including its treatment of quality values. It does not filter content types.
// Response bodies are buffered because Resolve produces a buffered proxy result.
func Compress(options CompressionOptions) Middleware {
	encoding, threshold := options.Encoding, float64(1024)
	if encoding == "" {
		encoding = "gzip"
	}
	if options.Threshold != nil {
		threshold = *options.Threshold
	}
	return func(request *RequestContext, next Next) error {
		if err := next(); err != nil {
			return err
		}
		if err := request.Context.Err(); err != nil {
			return err
		}
		response := request.Response
		if response.Header.Get("Transfer-Encoding") == "chunked" {
			return nil
		}
		hasBody := response.Body != nil && response.Body != nethttp.NoBody
		if hasBody && !hasHeader(response.Header, "Content-Length") {
			body := response.Body
			response.Body = nethttp.NoBody
			data, err := readOwnedBody(body)
			if err != nil {
				return err
			}
			response.Body = ioBody(data)
			response.Header.Set("Content-Length", strconv.Itoa(len(data)))
		}
		if err := request.Context.Err(); err != nil {
			return err
		}
		accepted := request.Request.Header.Get("Accept-Encoding")
		if !hasHeader(request.Request.Header, "Accept-Encoding") {
			accepted = "*"
		}
		length := response.Header.Get("Content-Length")
		if !hasBody || request.Request.Method == nethttp.MethodHead ||
			hasHeader(response.Header, "Content-Encoding") || hasHeader(response.Header, "Transfer-Encoding") ||
			strings.Contains(accepted, "identity") ||
			!(strings.Contains(accepted, encoding) || strings.Contains(accepted, "*")) ||
			length != "" && !(commons.ParseNumber(length) > threshold) ||
			noTransform(response.Header.Get("Cache-Control")) {
			return nil
		}
		var output bytes.Buffer
		var writer io.WriteCloser
		switch encoding {
		case "gzip":
			writer = gzip.NewWriter(&output)
		case "deflate":
			writer = zlib.NewWriter(&output)
		default:
			return fmt.Errorf("unsupported compression encoding %q", encoding)
		}
		body := response.Body
		response.Body = nethttp.NoBody
		err := compressBody(request.Context, writer, body)
		if err != nil {
			return err
		}
		response.Body = ioBody(output.Bytes())
		response.ContentLength = -1
		response.Header.Del("Content-Length")
		response.Header.Set("Content-Encoding", encoding)
		return nil
	}
}

func noTransform(value string) bool {
	for _, directive := range strings.Split(value, ",") {
		if strings.EqualFold(commons.TrimSpace(directive), "no-transform") {
			return true
		}
	}
	return false
}

type contextReader struct {
	context context.Context
	reader  io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.context.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func compressBody(ctx context.Context, writer io.WriteCloser, body io.ReadCloser) (err error) {
	defer func() {
		if closeErr := body.Close(); err == nil {
			err = closeErr
		}
	}()
	defer func() {
		if closeErr := writer.Close(); err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(writer, contextReader{context: ctx, reader: body})
	if err == nil {
		err = ctx.Err()
	}
	return err
}
