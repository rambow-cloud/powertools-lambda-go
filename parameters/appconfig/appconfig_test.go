package appconfig

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/appconfigdata"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

type fakeClient struct {
	start func(context.Context, *sdk.StartConfigurationSessionInput) (*sdk.StartConfigurationSessionOutput, error)
	get   func(context.Context, *sdk.GetLatestConfigurationInput) (*sdk.GetLatestConfigurationOutput, error)
}

func (f fakeClient) StartConfigurationSession(ctx context.Context, input *sdk.StartConfigurationSessionInput, _ ...func(*sdk.Options)) (*sdk.StartConfigurationSessionOutput, error) {
	return f.start(ctx, input)
}
func (f fakeClient) GetLatestConfiguration(ctx context.Context, input *sdk.GetLatestConfigurationInput, _ ...func(*sdk.Options)) (*sdk.GetLatestConfigurationOutput, error) {
	return f.get(ctx, input)
}

func TestTokenRotationExpiryAndUnchangedValue(t *testing.T) {
	now := time.Unix(100, 0)
	starts, gets := 0, 0
	expectedToken := ""
	client := fakeClient{
		start: func(_ context.Context, input *sdk.StartConfigurationSessionInput) (*sdk.StartConfigurationSessionOutput, error) {
			starts++
			if aws.ToString(input.ApplicationIdentifier) != "orders" || aws.ToString(input.EnvironmentIdentifier) != "test" || aws.ToString(input.ConfigurationProfileIdentifier) != "flags" || aws.ToInt32(input.RequiredMinimumPollIntervalInSeconds) != 15 {
				t.Fatalf("session input: %#v", input)
			}
			expectedToken = fmt.Sprintf("session-%d", starts)
			return &sdk.StartConfigurationSessionOutput{InitialConfigurationToken: aws.String(expectedToken)}, nil
		},
		get: func(_ context.Context, input *sdk.GetLatestConfigurationInput) (*sdk.GetLatestConfigurationOutput, error) {
			if aws.ToString(input.ConfigurationToken) != expectedToken {
				t.Fatalf("reused token: %s, want %s", aws.ToString(input.ConfigurationToken), expectedToken)
			}
			gets++
			expectedToken = fmt.Sprintf("poll-%d", gets)
			out := &sdk.GetLatestConfigurationOutput{NextPollConfigurationToken: aws.String(expectedToken)}
			if gets == 1 {
				out.Configuration = []byte(`{"enabled":true}`)
			}
			return out, nil
		},
	}
	p, err := New(client, Config{Application: "orders", Environment: "test", Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	opts := GetOptions{Options: parameters.Options{Transform: parameters.JSON}, SDKOptions: &sdk.StartConfigurationSessionInput{RequiredMinimumPollIntervalInSeconds: aws.Int32(15)}}
	for range 2 {
		value, err := p.Get(context.Background(), "flags", opts)
		if err != nil || !reflect.DeepEqual(value, map[string]any{"enabled": true}) {
			t.Fatalf("value: %v %v", value, err)
		}
	}
	if starts != 1 || gets != 1 {
		t.Fatal("cache miss")
	}
	p.ClearCache()
	value, err := p.Get(context.Background(), "flags", opts)
	if err != nil || !reflect.DeepEqual(value, map[string]any{"enabled": true}) || gets != 2 || starts != 1 {
		t.Fatalf("empty update lost previous value: %v %v", value, err)
	}
	now = now.Add(23*time.Hour + 45*time.Minute)
	_, err = p.Get(context.Background(), "flags", opts)
	if err != nil || starts != 2 || gets != 3 {
		t.Fatal("token expiry did not start new session")
	}
}

func TestConcurrentSingleUseTokensAndCancellation(t *testing.T) {
	var starts, gets atomic.Int32
	var inFlight atomic.Bool
	var overlap atomic.Bool
	client := fakeClient{
		start: func(context.Context, *sdk.StartConfigurationSessionInput) (*sdk.StartConfigurationSessionOutput, error) {
			starts.Add(1)
			return &sdk.StartConfigurationSessionOutput{InitialConfigurationToken: aws.String("token-0")}, nil
		},
		get: func(_ context.Context, input *sdk.GetLatestConfigurationInput) (*sdk.GetLatestConfigurationOutput, error) {
			if inFlight.Swap(true) {
				overlap.Store(true)
			}
			defer inFlight.Store(false)
			n := gets.Add(1)
			if aws.ToString(input.ConfigurationToken) != fmt.Sprintf("token-%d", n-1) {
				overlap.Store(true)
			}
			return &sdk.GetLatestConfigurationOutput{NextPollConfigurationToken: aws.String(fmt.Sprintf("token-%d", n)), Configuration: []byte("value")}, nil
		},
	}
	p, _ := New(client, Config{Application: "app", Environment: "test"})
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			value, err := p.Get(context.Background(), "flags", GetOptions{Options: parameters.Options{ForceFetch: true}})
			if err != nil || string(value.([]byte)) != "value" {
				t.Errorf("concurrent get: %v %v", value, err)
			}
		})
	}
	wg.Wait()
	if overlap.Load() || starts.Load() != 1 || gets.Load() != 100 {
		t.Fatalf("token serialization: overlap=%v starts=%d gets=%d", overlap.Load(), starts.Load(), gets.Load())
	}
	p.mu.Lock()
	state := p.sessions["flags"]
	p.mu.Unlock()
	state.gate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := p.Get(ctx, "flags", GetOptions{Options: parameters.Options{ForceFetch: true}})
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled waiter blocked")
	}
	<-state.gate
}

func TestFailedPollRestartsSessionAndMissing(t *testing.T) {
	starts := 0
	boom := errors.New("connection interrupted")
	client := fakeClient{
		start: func(context.Context, *sdk.StartConfigurationSessionInput) (*sdk.StartConfigurationSessionOutput, error) {
			starts++
			return &sdk.StartConfigurationSessionOutput{InitialConfigurationToken: aws.String("token")}, nil
		},
		get: func(context.Context, *sdk.GetLatestConfigurationInput) (*sdk.GetLatestConfigurationOutput, error) {
			if starts == 1 {
				return nil, boom
			}
			return &sdk.GetLatestConfigurationOutput{}, nil
		},
	}
	p, _ := New(client, Config{Application: "app", Environment: "test"})
	_, err := p.Get(context.Background(), "flags", GetOptions{})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	_, err = p.Get(context.Background(), "flags", GetOptions{Options: parameters.Options{ThrowOnMissing: true}})
	var missing *parameters.ParameterNotFoundError
	if starts != 2 || !errors.As(err, &missing) {
		t.Fatalf("restart or missing: starts=%d err=%v", starts, err)
	}
}
