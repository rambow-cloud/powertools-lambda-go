package appsyncevents

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestIndividualAuthorizationErrors(t *testing.T) {
	cause := errors.New("identity rejected")
	denied := &UnauthorizedException{Message: "denied", Cause: cause}
	wrapped := fmt.Errorf("publication: %w", denied)
	for _, test := range []struct {
		name  string
		err   error
		panic bool
	}{
		{"direct", denied, false},
		{"wrapped", wrapped, false},
		{"panic", denied, true},
		{"wrapped-panic", wrapped, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := New(quiet())
			app.OnPublish("/*", func(context.Context, any, Event) (any, error) {
				if test.panic {
					panic(test.err)
				}
				return nil, test.err
			})
			result, err := app.Resolve(context.Background(), event("/default", "PUBLISH", 1))
			var unauthorized *UnauthorizedException
			if result != nil || err != test.err || !errors.Is(err, cause) || !errors.As(err, &unauthorized) || unauthorized != denied {
				t.Fatalf("authorization identity/chain lost: result=%v error=%v", result, err)
			}
		})
	}
}

func TestMixedAuthorizationWaitsForWorkers(t *testing.T) {
	first := fmt.Errorf("first: %w", &UnauthorizedError{Message: "first denied"})
	second := &UnauthorizedError{Message: "second denied"}
	ordinary := errors.New("ordinary failure")
	started, finished := make(chan int, 4), make(chan int, 4)
	release, secondDiagnosed, firstDiagnosed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(finish)
	var mu sync.Mutex
	var diagnostics []error
	app := New(Options{Diagnostic: func(_ context.Context, level, _ string, err error) {
		if level != "error" {
			return
		}
		mu.Lock()
		diagnostics = append(diagnostics, err)
		mu.Unlock()
		if err == second {
			close(secondDiagnosed)
		} else if err == first {
			close(firstDiagnosed)
		}
	}})
	app.OnPublish("/*", func(_ context.Context, value any, _ Event) (any, error) {
		i := value.(int)
		started <- i
		defer func() { finished <- i }()
		switch i {
		case 0:
			<-secondDiagnosed // Complete the later denial first.
			return nil, first
		case 1:
			return nil, second
		case 2:
			return nil, ordinary
		default:
			<-release
			return value, nil
		}
	})
	type outcome struct {
		value any
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		value, err := app.Resolve(context.Background(), event("/default", "PUBLISH", 0, 1, 2, 3))
		done <- outcome{value, err}
	}()
	for range 4 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("handlers did not start concurrently")
		}
	}
	select {
	case <-firstDiagnosed:
	case <-time.After(5 * time.Second):
		t.Fatal("authorization failures were not processed")
	}
	select {
	case result := <-done:
		t.Fatalf("returned while a worker was blocked: %+v", result)
	case <-time.After(100 * time.Millisecond):
	}
	finish()
	select {
	case result := <-done:
		if result.value != nil || result.err != first {
			t.Fatalf("want first input denial without a partial response: %+v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resolver did not finish after releasing the worker")
	}
	if len(finished) != 4 {
		t.Fatalf("returned before all callbacks finished: %d", len(finished))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(diagnostics) != 3 {
		t.Fatalf("want both denials and the ordinary error diagnosed: %v", diagnostics)
	}
}
