// Package main exercises the real Go Lambda SDK against a local Runtime API fixture.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	nethttp "net/http"
	"os"
	"sync/atomic"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	httpapi "github.com/rambow-cloud/powertools-lambda-go/eventhandler/http"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
	"github.com/rambow-cloud/powertools-lambda-go/tracer"
)

type call struct {
	event  json.RawMessage
	writer io.Writer
}
type bodyState struct {
	closes atomic.Int32
	reads  atomic.Int32
}
type chunkReader struct {
	ctx      context.Context
	mode, id string
	state    *bodyState
	pending  []byte
	step     int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(r.pending) == 0 {
		switch r.step {
		case 0:
			r.pending = []byte("chunk-one")
		case 1:
			request, err := nethttp.NewRequestWithContext(r.ctx, "GET", os.Getenv("STREAM_FIXTURE")+"/stream/gate/"+r.id, nil)
			if err != nil {
				return 0, err
			}
			response, err := nethttp.DefaultClient.Do(request)
			if err != nil {
				return 0, err
			}
			_ = response.Body.Close()
			if response.StatusCode != 204 {
				return 0, fmt.Errorf("first chunk was not observed: %d", response.StatusCode)
			}
			switch r.mode {
			case "read-error":
				return 0, errors.New("stream read failure")
			case "panic":
				panic("stream read panic")
			case "deadline":
				<-r.ctx.Done()
				return 0, r.ctx.Err()
			}
			r.pending = []byte("chunk-two")
		default:
			return 0, io.EOF
		}
		r.step++
	}
	r.state.reads.Add(1)
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
func (r *chunkReader) Close() error {
	r.state.closes.Add(1)
	if r.mode == "close-error" {
		return errors.New("stream close failure")
	}
	return nil
}

func main() {
	requestLog := logger.New(logger.WithServiceName("streaming-probe"))
	trace, err := tracer.New(tracer.WithServiceName("streaming-probe"))
	if err != nil {
		log.Fatal(err)
	}
	app := httpapi.New(httpapi.Options{})
	app.Use(httpapi.CORS(httpapi.CORSOptions{Origins: []string{"https://app.test"}}), httpapi.Compress(httpapi.CompressionOptions{}))
	type stateKey struct{}
	if err := app.Get("/stream/:mode", func(request *httpapi.RequestContext) (any, error) {
		invocation, _ := lambdacontext.FromContext(request.Context)
		request.Response.Header.Set("X-Request-ID", invocation.AwsRequestID)
		mode := request.Params["mode"]
		if mode == "no-content" {
			return httpapi.Response{StatusCode: 204}, nil
		}
		if mode == "http-error" {
			return nil, httpapi.NewHTTPError(400, "stream handler rejected")
		}
		state := request.Context.Value(stateKey{}).(*bodyState)
		return httpapi.Response{StatusCode: 200, Headers: nethttp.Header{"Content-Type": []string{"text/plain"}}, Body: &chunkReader{ctx: request.Context, mode: mode, id: invocation.AwsRequestID, state: state}}, nil
	}); err != nil {
		log.Fatal(err)
	}
	complete := tracer.WrapHandler(trace, logger.WrapHandler(requestLog, func(ctx context.Context, input call) (struct{}, error) {
		state := &bodyState{}
		ctx = context.WithValue(ctx, stateKey{}, state)
		defer func() {
			_ = requestLog.WithContext(ctx).Info("stream complete", logger.Fields{"body_closes": state.closes.Load(), "body_reads": state.reads.Load()})
		}()
		return struct{}{}, app.ResolveStream(ctx, input.event, input.writer)
	}), tracer.HandlerOptions{Name: "stream-invocation", DisableCaptureResponse: true})
	lambda.Start(httpapi.Streamify(func(ctx context.Context, event json.RawMessage, destination io.Writer) error {
		_, err := complete(ctx, call{event: event, writer: destination})
		return err
	}))
}
