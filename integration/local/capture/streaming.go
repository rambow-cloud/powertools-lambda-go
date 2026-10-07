package main

import (
	"bufio"
	"encoding/base64"
	json "encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type streamInvocation struct {
	ID    string `json:"id"`
	Mode  string `json:"mode"`
	Index int    `json:"index"`
	first chan struct{}
}
type streamRuntime struct {
	mu      sync.Mutex
	queue   chan *streamInvocation
	active  map[string]*streamInvocation
	results map[string]map[string]any
}

func newStreamRuntime() *streamRuntime {
	return &streamRuntime{queue: make(chan *streamInvocation, 1), active: map[string]*streamInvocation{}, results: map[string]map[string]any{}}
}

func (s *streamRuntime) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/stream/invoke":
		var invocation streamInvocation
		if err := json.UnmarshalRead(r.Body, &invocation); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		invocation.first = make(chan struct{})
		s.mu.Lock()
		s.active[invocation.ID] = &invocation
		s.mu.Unlock()
		select {
		case s.queue <- &invocation:
			w.WriteHeader(202)
		case <-r.Context().Done():
		}
	case strings.HasPrefix(r.URL.Path, "/stream/result/"):
		s.mu.Lock()
		result := s.results[strings.TrimPrefix(r.URL.Path, "/stream/result/")]
		s.mu.Unlock()
		if result == nil {
			result = map[string]any{"completed": false}
		}
		_ = json.MarshalWrite(w, result)
	case strings.HasPrefix(r.URL.Path, "/stream/gate/"):
		s.mu.Lock()
		invocation := s.active[strings.TrimPrefix(r.URL.Path, "/stream/gate/")]
		s.mu.Unlock()
		if invocation == nil {
			http.Error(w, "missing invocation", 404)
			return
		}
		select {
		case <-invocation.first:
			w.WriteHeader(204)
		case <-r.Context().Done():
		}
	case strings.HasSuffix(r.URL.Path, "/next"):
		select {
		case invocation := <-s.queue:
			deadline := time.Now().Add(20 * time.Second)
			if invocation.Mode == "deadline" {
				deadline = time.Now().Add(1500 * time.Millisecond)
			}
			w.Header().Set("Lambda-Runtime-Aws-Request-Id", invocation.ID)
			w.Header().Set("Lambda-Runtime-Deadline-Ms", fmt.Sprint(deadline.UnixMilli()))
			w.Header().Set("Lambda-Runtime-Invoked-Function-Arn", "arn:aws:lambda:ap-east-1:012345678912:function:stream-probe")
			w.Header().Set("Lambda-Runtime-Trace-Id", fmt.Sprintf("Root=1-6aaab31e-%024x;Parent=0000000000000001;Sampled=1", invocation.Index+1))
			var event any = map[string]any{"version": "2.0", "routeKey": "$default", "rawPath": "/stream/" + invocation.Mode, "rawQueryString": "", "headers": map[string]string{"origin": "https://app.test"}, "requestContext": map[string]any{"domainName": "stream.example.test", "http": map[string]string{"method": "GET"}}, "isBase64Encoded": false}
			if invocation.Mode == "invalid-event" {
				event = map[string]any{}
			}
			_ = json.MarshalWrite(w, event)
		case <-r.Context().Done():
		}
	default:
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 5 {
			http.NotFound(w, r)
			return
		}
		id, action := parts[3], parts[4]
		s.mu.Lock()
		invocation := s.active[id]
		s.mu.Unlock()
		if invocation == nil {
			http.Error(w, "missing invocation", 404)
			return
		}
		result := map[string]any{"completed": true, "id": id, "action": action, "content_type": r.Header.Get("Content-Type"), "transfer_encoding": r.TransferEncoding, "response_mode": r.Header.Get("Lambda-Runtime-Function-Response-Mode")}
		if action == "error" {
			data, err := io.ReadAll(r.Body)
			result["body"] = string(data)
			if err != nil {
				result["read_error"] = err.Error()
			}
		} else {
			reader := bufio.NewReader(r.Body)
			metadata, err := reader.ReadBytes(0)
			if err == nil {
				var header any
				err = json.Unmarshal(metadata[:len(metadata)-1], &header)
				result["metadata"] = header
			}
			delimiter := make([]byte, 7)
			if err == nil {
				_, err = io.ReadFull(reader, delimiter)
				result["delimiter_valid"] = string(delimiter) == string(make([]byte, 7))
			}
			var payload []byte
			if err == nil {
				first := make([]byte, len("chunk-one"))
				n, readErr := io.ReadFull(reader, first)
				payload = append(payload, first[:n]...)
				result["first_chunk"] = string(first[:n])
				close(invocation.first)
				if readErr == nil {
					rest, readErr := io.ReadAll(reader)
					payload = append(payload, rest...)
					err = readErr
				} else if readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
					err = readErr
				}
			}
			if err != nil {
				result["read_error"] = err.Error()
			}
			result["body_base64"] = base64.StdEncoding.EncodeToString(payload)
			result["trailers"] = r.Trailer
		}
		s.mu.Lock()
		s.results[id] = result
		s.mu.Unlock()
		w.WriteHeader(202)
	}
}
