package http

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestErrorHandlerRedispatchBounded(t *testing.T) {
	for _, mode := range []string{"fresh-same-type", "cycle", "http-fallback", "error-fallback", "generic-error", "same-instance", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			app := New(Options{})
			first, second := 0, 0
			name := "BadRequestError"
			if mode == "http-fallback" {
				name = "HttpError"
			} else if mode == "error-fallback" {
				name = "Error"
			}
			app.OnError(name, func(err error, _ *RequestContext) (any, error) {
				first++
				// Keep the old defect finite instead of exhausting the process stack.
				if first+second > 32 || mode == "same-instance" {
					return nil, err
				}
				if mode == "generic-error" {
					return nil, errors.New("ordinary callback failure")
				}
				if mode == "cancel" {
					cancel()
				}
				if mode == "cycle" || mode == "http-fallback" || mode == "error-fallback" {
					return nil, NewHTTPError(401, "next error")
				}
				return nil, NewHTTPError(400, "fresh error")
			})
			if mode == "cycle" {
				app.OnError("UnauthorizedError", func(err error, _ *RequestContext) (any, error) {
					second++
					if first+second > 32 {
						return nil, err
					}
					return nil, NewHTTPError(400, "cycle back")
				})
			}
			if err := app.Get("/items", func(*RequestContext) (any, error) {
				return nil, NewHTTPError(400, "initial error")
			}); err != nil {
				t.Fatal(err)
			}
			result, err := app.Resolve(ctx, testEvent("/items"))
			if mode == "cancel" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost: %+v / %v", result, err)
				}
			} else if err != nil || result.StatusCode != 500 {
				t.Fatalf("fallback: %+v / %v", result, err)
			}
			wantSecond := 0
			if mode == "cycle" {
				wantSecond = 1
			}
			if first != 1 || second != wantSecond {
				t.Fatalf("callback redispatch was not bounded: first=%d second=%d; want 1/%d", first, second, wantSecond)
			}
		})
	}
}

func TestFiniteErrorHandlerRedispatch(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			app := New(Options{})
			first, second := 0, 0
			value := map[string]any{"message": "handled"}
			app.OnError("BadRequestError", func(error, *RequestContext) (any, error) {
				first++
				return nil, fmt.Errorf("wrapped: %w", NewHTTPError(404, "not found"))
			})
			app.NotFound(func(error, *RequestContext) (any, error) {
				second++
				if explicit {
					return Response{StatusCode: 202, Body: "accepted"}, nil
				}
				return value, nil
			})
			if err := app.Get("/items", func(*RequestContext) (any, error) {
				return nil, NewHTTPError(400, "initial")
			}); err != nil {
				t.Fatal(err)
			}
			result, err := app.Resolve(context.Background(), testEvent("/items"))
			wantStatus := 404
			if explicit {
				wantStatus = 202
			}
			if err != nil || result.StatusCode != wantStatus || first != 1 || second != 1 {
				t.Fatalf("finite chain: %+v / %v, calls=%d/%d", result, err, first, second)
			}
			if _, mutated := value["statusCode"]; mutated {
				t.Fatal("error-handler map mutated")
			}
		})
	}
}
