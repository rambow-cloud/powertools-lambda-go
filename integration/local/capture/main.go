// Package main records OTLP requests and serves deterministic downstream fixtures.
// It is test infrastructure, not a production collector or an AWS service emulator.
package main

import (
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	collector "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func main() {
	var mu sync.Mutex
	batches := []jsonv1.RawMessage{}
	requests := []map[string]string{}
	appConfigPolls := 0
	mux := http.NewServeMux()
	streams := newStreamRuntime()
	mux.Handle("/stream/", streams)
	mux.Handle("/2018-06-01/runtime/invocation/", streams)
	idempotency := newIdempotencyFixture()
	mux.Handle("/idempotency", idempotency)
	mux.HandleFunc("/v1/traces", func(w http.ResponseWriter, r *http.Request) {
		var request collector.ExportTraceServiceRequest
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err = proto.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		data, err := protojson.Marshal(&request)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		mu.Lock()
		batches = append(batches, data)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/snapshot", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.MarshalWrite(w, map[string]any{"batches": batches, "requests": requests, "idempotency": idempotency.snapshot()})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, map[string]string{"path": r.URL.Path, "target": r.Header.Get("X-Amz-Target"), "traceparent": r.Header.Get("Traceparent"), "xray": r.Header.Get("X-Amzn-Trace-Id"), "user_agent": r.Header.Get("User-Agent"), "metadata_authenticated": fmt.Sprint(r.Header.Get("Authorization") == "Bearer local-metadata-token"), "signature_valid": fmt.Sprint(r.URL.Path == "/fixture.txt" && verifyFixtureSignature(r))})
		mu.Unlock()
		switch {
		case r.Method == "GET" && r.URL.Path == "/2026-01-15/metadata/execution-environment":
			if r.Header.Get("Authorization") != "Bearer local-metadata-token" {
				http.Error(w, "unauthorized", 401)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"AvailabilityZoneID":"ape1-az1","future":{"enabled":true}}`)
		case r.Method == "POST" && r.Header.Get("X-Amz-Target") == "AmazonSSM.GetParameter":
			w.Header().Set("Content-Type", "application/x-amz-json-1.1")
			_, _ = io.WriteString(w, `{"Parameter":{"Name":"/local/flags","Value":"{\"enabled\":true}"}}`)
		case r.Method == "POST" && r.Header.Get("X-Amz-Target") == "secretsmanager.GetSecretValue":
			w.Header().Set("Content-Type", "application/x-amz-json-1.1")
			_, _ = io.WriteString(w, `{"SecretString":"{\"enabled\":true}"}`)
		case r.Method == "POST" && r.URL.Path == "/configurationsessions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"InitialConfigurationToken":"local-token-0"}`)
		case r.Method == "GET" && r.URL.Path == "/configuration":
			mu.Lock()
			defer mu.Unlock()
			if r.URL.Query().Get("configuration_token") != fmt.Sprintf("local-token-%d", appConfigPolls) {
				http.Error(w, "token reused", 400)
				return
			}
			appConfigPolls++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Next-Poll-Configuration-Token", fmt.Sprintf("local-token-%d", appConfigPolls))
			w.Header().Set("Next-Poll-Interval-In-Seconds", "15")
			if appConfigPolls == 1 {
				_, _ = io.WriteString(w, `{"enabled":true}`)
			}
		case r.Method == "GET" && r.URL.Path == "/applications/local/environments/test/configurations/flags":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"enabled":true}`)
		case r.Method == "GET" && r.URL.Path == "/fixture.txt":
			_, _ = io.WriteString(w, "powertools-integration")
		case r.Method == "POST" && r.Header.Get("X-Amz-Target") == "DynamoDB_20120810.GetItem":
			w.Header().Set("Content-Type", "application/x-amz-json-1.0")
			w.Header().Set("X-Amzn-Requestid", "local-fixture-request")
			_, _ = io.WriteString(w, `{"Item":{"value":{"S":"{\"enabled\":true}"}}}`)
		default:
			http.NotFound(w, r)
		}
	})
	log.Fatal(http.ListenAndServe(":4318", mux))
}
