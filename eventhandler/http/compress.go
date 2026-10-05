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

// Compress selects the configured coding using exact tokens and quality weights.
// An explicitly preferred identity skips compression; equal weights may compress.
// It does not filter content types or change status when no coding is selected.
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
		accepted := strings.Join(request.Request.Header.Values("Accept-Encoding"), ",")
		if !hasHeader(request.Request.Header, "Accept-Encoding") {
			accepted = "*"
		}
		length := response.Header.Get("Content-Length")
		if !hasBody || request.Request.Method == nethttp.MethodHead ||
			hasHeader(response.Header, "Content-Encoding") || hasHeader(response.Header, "Transfer-Encoding") ||
			!acceptsCompression(accepted, encoding) ||
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

func acceptsCompression(accepted, encoding string) bool {
	weights := map[string]float64{}
	for _, entry := range strings.Split(accepted, ",") {
		coding, parameter, weighted := strings.Cut(entry, ";")
		coding = strings.ToLower(strings.TrimSpace(coding))
		if coding == "" {
			continue
		}
		weight := float64(1)
		if weighted {
			name, value, present := strings.Cut(parameter, "=")
			if !present || !strings.EqualFold(strings.TrimSpace(name), "q") {
				weight = 0
			} else {
				weight = compressionQuality(strings.TrimSpace(value))
			}
		}
		// Repeated offers use their highest quality, never a substring match.
		if previous, present := weights[coding]; !present || weight > previous {
			weights[coding] = weight
		}
	}
	weight, specific := weights[strings.ToLower(encoding)]
	if !specific {
		weight = weights["*"]
	}
	// Without an explicit identity preference, an offered coding may be used.
	return weight > 0 && weight >= weights["identity"]
}

// RFC 9110 qvalues are 0/1 with up to three decimal digits; 1 has only zeroes.
// Malformed weights are excluded from compression selection.
func compressionQuality(value string) float64 {
	if value == "" || value[0] != '0' && value[0] != '1' {
		return 0
	}
	if len(value) > 1 {
		if value[1] != '.' || len(value) > 5 {
			return 0
		}
		for _, digit := range value[2:] {
			if digit < '0' || digit > '9' || value[0] == '1' && digit != '0' {
				return 0
			}
		}
	}
	weight, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return weight
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
