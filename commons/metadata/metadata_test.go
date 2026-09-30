package metadata_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons/metadata"
)

func TestMetadataReference(t *testing.T) {
	data, err := os.ReadFile("testdata/typescript-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Metadata struct {
			First, Second, Third, Local map[string]any
			Requests                    []struct{ URL, Authorization string }
		}
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	var requests []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, map[string]string{"url": "http://metadata.local:9001" + r.URL.Path, "authorization": r.Header.Get("Authorization")})
		_ = json.NewEncoder(w).Encode(f.Metadata.First)
	}))
	defer server.Close()
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "on-demand")
	t.Setenv("POWERTOOLS_DEV", "false")
	t.Setenv("AWS_LAMBDA_METADATA_API", strings.TrimPrefix(server.URL, "http://"))
	t.Setenv("AWS_LAMBDA_METADATA_TOKEN", "fixture-token")
	metadata.ClearMetadataCache()
	t.Cleanup(metadata.ClearMetadataCache)
	for i, expected := range []map[string]any{f.Metadata.First, f.Metadata.Second, f.Metadata.Third} {
		if i == 2 {
			metadata.ClearMetadataCache()
		}
		value, err := metadata.GetMetadata(context.Background())
		if err != nil || !reflect.DeepEqual(value, expected) {
			t.Fatalf("metadata: %v %v", value, err)
		}
	}
	if len(requests) != len(f.Metadata.Requests) {
		t.Fatalf("request count: %d", len(requests))
	}
	for i, request := range requests {
		if request["url"] != f.Metadata.Requests[i].URL || request["authorization"] != f.Metadata.Requests[i].Authorization {
			t.Fatal(request)
		}
	}
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "unknown")
	value, err := metadata.GetMetadata(context.Background())
	if err != nil || !reflect.DeepEqual(value, f.Metadata.Local) {
		t.Fatalf("local result: %v %v", value, err)
	}
}

func TestMetadataConcurrentSnapshotsAndClear(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"future":{"enabled":true}}`)
	}))
	defer server.Close()
	client := metadata.New(metadata.Config{Endpoint: server.URL, Token: "test-token"})
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			value, err := client.Get(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			if value["future"].(map[string]any)["enabled"] != true {
				t.Error("snapshot changed")
			}
			value["future"].(map[string]any)["enabled"] = false
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent fetches: %d", calls.Load())
	}
	client.ClearCache()
	_, err := client.Get(context.Background())
	if err != nil || calls.Load() != 2 {
		t.Fatalf("clear: %d %v", calls.Load(), err)
	}
}

func TestMetadataFailuresTimeoutAndNoErrorCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(500)
			fmt.Fprint(w, "private-token-or-data")
		case 2:
			fmt.Fprint(w, `{} trailing`)
		case 3:
			fmt.Fprint(w, `{}`)
		default:
			fmt.Fprint(w, `{"AvailabilityZoneID":"ape1-az1"}`)
		}
	}))
	defer server.Close()
	client := metadata.New(metadata.Config{Endpoint: server.URL, Token: "private-token"})
	_, err := client.Get(context.Background())
	var typed *metadata.Error
	if !errors.As(err, &typed) || typed.StatusCode != 500 || strings.Contains(err.Error(), "private") {
		t.Fatal(err)
	}
	_, err = client.Get(context.Background())
	if err == nil {
		t.Fatal("trailing JSON accepted")
	}
	_, err = client.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	value, err := client.Get(context.Background())
	if err != nil || value["AvailabilityZoneID"] != "ape1-az1" || calls.Load() != 4 {
		t.Fatalf("failure/empty cache: %v %v", value, err)
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	client = metadata.New(metadata.Config{Endpoint: slow.URL, Token: "token"})
	_, err = client.Get(context.Background(), metadata.Options{Timeout: 20 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
}

func TestMetadataClearDuringFetchAndCancelledWaiter(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		fmt.Fprint(w, `{"value":1}`)
	}))
	defer server.Close()
	client := metadata.New(metadata.Config{Endpoint: server.URL, Token: "token"})
	done := make(chan error, 1)
	go func() { _, err := client.Get(context.Background()); done <- err }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := client.Get(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	client.ClearCache()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	_, err = client.Get(context.Background())
	if err != nil || calls.Load() != 2 {
		t.Fatalf("clear during fetch: %d %v", calls.Load(), err)
	}
}

func TestMetadataRejectsRedirects(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Store(true) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	client := metadata.New(metadata.Config{Endpoint: source.URL, Token: "secret-token"})
	_, err := client.Get(context.Background())
	var status *metadata.Error
	if !errors.As(err, &status) || status.StatusCode != 302 || leaked.Load() {
		t.Fatal("metadata redirect followed", err)
	}
}
