package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"sync"
)

// ResolveStream writes the Lambda HTTP integration metadata, eight zero bytes,
// then raw body bytes. It finishes copying and closing the body before returning.
// The caller owns the destination. Write/read errors after metadata are returned
// without attempting to replace the response with another HTTP error document.
func (r *Router) ResolveStream(ctx context.Context, event any, destination io.Writer) error {
	if destination == nil {
		return errors.New("stream destination must not be nil")
	}
	return r.resolve(ctx, event, true, func(request *RequestContext) error {
		response := request.Response
		headers := ProxyResponse{Headers: map[string]string{}}
		for name, values := range response.Header {
			name = strings.ToLower(name)
			if name == "set-cookie" && len(values) > 0 {
				headers.Headers[name] = values[len(values)-1]
			} else {
				headers.Headers[name] = strings.Join(values, ", ")
			}
		}
		// Headers-only metadata joins non-cookie values for every adapter.
		// The existing streaming contract retains only the final cookie.
		metadata, err := jsonBytes(struct {
			StatusCode int               `json:"statusCode"`
			Headers    map[string]string `json:"headers"`
		}{response.StatusCode, headers.Headers})
		if err != nil {
			return err
		}
		body := response.Body
		response.Body = nethttp.NoBody
		if body == nil {
			body = nethttp.NoBody
		}
		return writeStream(ctx, destination, append(metadata, make([]byte, 8)...), body)
	})
}

// streamBody serializes cancellation and ordinary closure of an owned reader.
type streamBody struct {
	io.ReadCloser
	once sync.Once
	err  error
}

func (b *streamBody) Close() error {
	b.once.Do(func() { b.err = b.ReadCloser.Close() })
	return b.err
}

func writeStream(ctx context.Context, destination io.Writer, metadata []byte, source io.ReadCloser) (err error) {
	body := &streamBody{ReadCloser: source}
	stop := context.AfterFunc(ctx, func() { _ = body.Close() })
	defer func() {
		stop()
		if closeErr := body.Close(); err == nil {
			err = closeErr
		}
	}()
	if err = ctx.Err(); err != nil {
		return err
	}
	n, err := destination.Write(metadata)
	if err != nil {
		return err
	}
	if n != len(metadata) {
		return io.ErrShortWrite
	}
	_, err = io.Copy(destination, contextReader{context: ctx, reader: body})
	if cancelled := ctx.Err(); cancelled != nil {
		return cancelled
	}
	return err
}

// StreamPanicError reports a panic from the producer goroutine after synchronous
// middleware and observability defers have run. Value retains the original panic.
type StreamPanicError struct{ Value any }

func (e *StreamPanicError) Error() string { return fmt.Sprintf("stream handler panic: %v", e.Value) }
func (e *StreamPanicError) Unwrap() error {
	err, _ := e.Value.(error)
	return err
}

// ResponseStream is the native aws-lambda-go reader contract. Close cancels the
// producer and waits for its cleanup. Handlers/readers must cooperate with context
// cancellation; arbitrary blocking application code cannot be forcibly stopped.
type ResponseStream struct {
	reader *io.PipeReader
	cancel context.CancelFunc
	done   chan struct{}
	err    error
	once   sync.Once
}

func (s *ResponseStream) Read(p []byte) (int, error) {
	n, err := s.reader.Read(p)
	if err != nil {
		// Cancellation can release a pipe operation before invocation defers
		// finish. Keep the runtime consuming until those defers complete.
		<-s.done
		if s.err != nil {
			err = s.err
		}
	}
	return n, err
}
func (s *ResponseStream) ContentType() string {
	return "application/vnd.awslambda.http-integration-response"
}
func (s *ResponseStream) MarshalJSON() ([]byte, error) {
	return nil, errors.New("HTTP response stream must be consumed as an io.Reader")
}
func (s *ResponseStream) Close() error {
	s.once.Do(func() {
		s.cancel()
		_ = s.reader.CloseWithError(context.Canceled)
		<-s.done
	})
	return s.err
}

type readyStreamWriter struct {
	writer *io.PipeWriter
	ready  chan struct{}
	once   sync.Once
}

func (w *readyStreamWriter) Write(p []byte) (int, error) {
	if len(p) > 0 {
		w.once.Do(func() { close(w.ready) })
	}
	return w.writer.Write(p)
}

// Streamify adapts a synchronous stream-writing handler to lambda.Start. Place
// Logger/Tracer wrappers inside handler so cleanup covers the entire body copy.
// Errors before the first write are invocation errors; subsequent errors reach
// the Lambda SDK through Read and can be reported as Runtime API error trailers.
func Streamify(handler func(context.Context, json.RawMessage, io.Writer) error) func(context.Context, json.RawMessage) (*ResponseStream, error) {
	return func(ctx context.Context, event json.RawMessage) (*ResponseStream, error) {
		if handler == nil {
			return nil, errors.New("stream handler must not be nil")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithCancel(ctx)
		reader, writer := io.Pipe()
		stream := &ResponseStream{reader: reader, cancel: cancel, done: make(chan struct{})}
		ready := &readyStreamWriter{writer: writer, ready: make(chan struct{})}
		snapshot := append(json.RawMessage(nil), event...)
		stop := context.AfterFunc(ctx, func() {
			_ = writer.CloseWithError(ctx.Err())
		})
		go func() {
			var err error
			defer func() {
				if value := recover(); value != nil {
					err = &StreamPanicError{Value: value}
				}
				stop()
				stream.err = err
				_ = writer.CloseWithError(err)
				close(stream.done)
			}()
			err = handler(ctx, snapshot, ready)
		}()
		select {
		case <-ready.ready:
			return stream, nil
		case <-stream.done:
			_ = stream.Close()
			if stream.err != nil {
				return nil, stream.err
			}
			return nil, errors.New("stream handler returned without writing a response")
		case <-ctx.Done():
			_ = stream.Close()
			return nil, ctx.Err()
		}
	}
}
