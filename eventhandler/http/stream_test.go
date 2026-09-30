package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type stagedBody struct {
	step    int
	blocked chan struct{}
	release chan struct{}
	closed  chan struct{}
	closes  atomic.Int32
	once    sync.Once
}

func newStagedBody() *stagedBody {
	return &stagedBody{blocked: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
}

func (b *stagedBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	switch b.step {
	case 0:
		b.step++
		p[0] = 'a'
		return 1, nil
	case 1:
		b.step++
		close(b.blocked)
		select {
		case <-b.release:
			p[0] = 'b'
			return 1, nil
		case <-b.closed:
			return 0, context.Canceled
		}
	default:
		return 0, io.EOF
	}
}
func (b *stagedBody) Close() error {
	b.closes.Add(1)
	b.once.Do(func() { close(b.closed) })
	return nil
}

func readStreamHeader(t *testing.T, source *bufio.Reader) map[string]any {
	t.Helper()
	metadata, err := source.ReadBytes(0)
	if err != nil {
		t.Fatal(err)
	}
	delimiter := make([]byte, 7)
	if _, err := io.ReadFull(source, delimiter); err != nil || !bytes.Equal(delimiter, make([]byte, 7)) {
		t.Fatalf("delimiter: %x, %v", delimiter, err)
	}
	var result map[string]any
	if err := json.Unmarshal(metadata[:len(metadata)-1], &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestStreamingFirstByteAndCancellation(t *testing.T) {
	for _, cancelConsumer := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelConsumer), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			body := newStagedBody()
			app := New(Options{})
			if err := app.Get("/items/a", func(*RequestContext) (any, error) {
				return &nethttp.Response{StatusCode: 201, Header: make(nethttp.Header), Body: body}, nil
			}); err != nil {
				t.Fatal(err)
			}
			var cleaned atomic.Bool
			handler := Streamify(func(ctx context.Context, event json.RawMessage, writer io.Writer) error {
				defer cleaned.Store(true)
				return app.ResolveStream(ctx, event, writer)
			})
			stream, err := handler(ctx, testEvent("/items/a"))
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			reader := bufio.NewReader(stream)
			metadata := readStreamHeader(t, reader)
			first, err := reader.ReadByte()
			if err != nil || first != 'a' || metadata["statusCode"] != float64(201) || cleaned.Load() {
				t.Fatalf("first byte/cleanup: %q, %v, %v", first, err, cleaned.Load())
			}
			select {
			case <-body.blocked:
			case <-ctx.Done():
				t.Fatal("reader did not reach the blocked second chunk")
			}
			if cancelConsumer {
				if err := stream.Close(); !errors.Is(err, context.Canceled) {
					t.Fatalf("close cancellation: %v", err)
				}
			} else {
				close(body.release)
				rest, err := io.ReadAll(reader)
				if err != nil || string(rest) != "b" {
					t.Fatalf("remainder: %q, %v", rest, err)
				}
				if err := stream.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if !cleaned.Load() || body.closes.Load() != 1 {
				t.Fatalf("cleanup=%v closes=%d", cleaned.Load(), body.closes.Load())
			}
		})
	}
}

type errorStreamWriter struct {
	failure error
	limit   int
	bytes   int
}

func (w *errorStreamWriter) Write(p []byte) (int, error) {
	if w.bytes >= w.limit {
		return 0, w.failure
	}
	w.bytes += len(p)
	return len(p), nil
}

func TestStreamFailureOwnership(t *testing.T) {
	for _, mode := range []string{"metadata-write", "body-write", "read", "close", "panic"} {
		t.Run(mode, func(t *testing.T) {
			failure := errors.New("stream failure")
			body := &observedBody{Reader: strings.NewReader("hello")}
			var destination io.Writer = io.Discard
			switch mode {
			case "metadata-write":
				destination = &errorStreamWriter{failure: failure}
			case "body-write":
				destination = &errorStreamWriter{failure: failure, limit: 1}
			case "read":
				body.Reader = failingCompressionReader{failure}
			case "close":
				body.closeError = failure
			case "panic":
				body.Reader = panicStreamReader{failure}
			}
			app := New(Options{})
			if err := app.Get("/items/a", func(*RequestContext) (any, error) { return body, nil }); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if mode == "panic" && recover() != failure {
					t.Error("panic identity changed")
				}
				if body.closed != 1 {
					t.Errorf("body closed %d times", body.closed)
				}
			}()
			if err := app.ResolveStream(context.Background(), testEvent("/items/a"), destination); !errors.Is(err, failure) {
				t.Fatalf("stream error: %v", err)
			}
		})
	}
}

type panicStreamReader struct{ value any }

func (r panicStreamReader) Read([]byte) (int, error) { panic(r.value) }

func TestStreamifyErrorStages(t *testing.T) {
	for _, late := range []bool{false, true} {
		for _, panicValue := range []bool{false, true} {
			t.Run(fmt.Sprintf("late=%t/panic=%t", late, panicValue), func(t *testing.T) {
				failure := errors.New("producer failed")
				cleaned := false
				handler := Streamify(func(_ context.Context, _ json.RawMessage, writer io.Writer) error {
					defer func() { cleaned = true }()
					if late {
						if _, err := writer.Write([]byte("prefix")); err != nil {
							return err
						}
					}
					if panicValue {
						panic(failure)
					}
					return failure
				})
				stream, err := handler(context.Background(), testEvent("/items/a"))
				if late {
					if err != nil {
						t.Fatal(err)
					}
					data, readErr := io.ReadAll(stream)
					if string(data) != "prefix" {
						t.Fatalf("lost successful bytes: %q", data)
					}
					err = readErr
					_ = stream.Close()
				}
				if !cleaned || !errors.Is(err, failure) {
					t.Fatalf("cleanup/error identity: %v, %v", cleaned, err)
				}
				var recovered *StreamPanicError
				if panicValue && (!errors.As(err, &recovered) || recovered.Value != failure) {
					t.Fatalf("panic mapping: %v", err)
				}
			})
		}
	}
}

type countedStream struct {
	remaining int
	maxRead   int
	closed    int
}

func (r *countedStream) Read(p []byte) (int, error) {
	if len(p) > r.maxRead {
		r.maxRead = len(p)
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	clear(p[:n])
	r.remaining -= n
	return n, nil
}
func (r *countedStream) Close() error { r.closed++; return nil }

func TestLargeStreamDoesNotMaterializeBody(t *testing.T) {
	body := &countedStream{remaining: 32 << 20}
	app := New(Options{})
	if err := app.Get("/items/a", func(*RequestContext) (any, error) { return body, nil }); err != nil {
		t.Fatal(err)
	}
	if err := app.ResolveStream(context.Background(), testEvent("/items/a"), io.Discard); err != nil {
		t.Fatal(err)
	}
	if body.remaining != 0 || body.closed != 1 || body.maxRead > 32<<10 {
		t.Fatalf("body buffering: %+v", body)
	}
}

func TestConcurrentStreamIsolation(t *testing.T) {
	app := New(Options{})
	if err := app.Get("/items/:id", func(request *RequestContext) (any, error) {
		return Response{StatusCode: 200, Body: strings.NewReader(request.Params["id"])}, nil
	}); err != nil {
		t.Fatal(err)
	}
	handler := Streamify(func(ctx context.Context, event json.RawMessage, writer io.Writer) error {
		return app.ResolveStream(ctx, event, writer)
	})
	var group sync.WaitGroup
	for index := range 32 {
		group.Go(func() {
			id := fmt.Sprint(index)
			stream, err := handler(context.Background(), testEvent("/items/"+id))
			if err != nil {
				t.Error(err)
				return
			}
			data, err := io.ReadAll(stream)
			closeErr := stream.Close()
			_, body := splitStream(t, data)
			if err != nil || closeErr != nil || string(body) != id {
				t.Errorf("stream isolation: %q, %v, %v", body, err, closeErr)
			}
		})
	}
	group.Wait()
}

func TestStreamCancellationWaitsForProducerCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleaning, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	handler := Streamify(func(ctx context.Context, _ json.RawMessage, writer io.Writer) error {
		if _, err := writer.Write([]byte("a")); err != nil {
			return err
		}
		<-ctx.Done()
		close(cleaning)
		<-release
		return ctx.Err()
	})
	stream, err := handler(ctx, testEvent("/items/a"))
	if err != nil {
		t.Fatal(err)
	}
	first := make([]byte, 1)
	if _, err := io.ReadFull(stream, first); err != nil {
		t.Fatal(err)
	}
	cancel()
	<-cleaning
	finished := make(chan error, 1)
	go func() { _, err := stream.Read(first); finished <- err }()
	select {
	case err := <-finished:
		t.Fatalf("runtime read completed before producer cleanup: %v", err)
	default:
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel cause: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runtime read did not finish after cleanup")
	}
	if err := stream.Close(); !errors.Is(err, context.Canceled) {
		t.Fatalf("close cause: %v", err)
	}
}
