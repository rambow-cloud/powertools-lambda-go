package main

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	persistence "github.com/rambow-cloud/powertools-lambda-go/idempotency/cache"
	"github.com/redis/go-redis/v9"
)

type cacheResult struct {
	WarmExecutions       int32          `json:"warm_executions"`
	WarmResponse         int32          `json:"warm_response"`
	ConcurrentExecutions int32          `json:"concurrent_executions"`
	ContendedCalls       int32          `json:"contended_calls"`
	Recovered            bool           `json:"recovered"`
	ValidationRejected   bool           `json:"validation_rejected"`
	TypeScriptResponse   map[string]any `json:"typescript_response"`
}

func newCacheProbe() func(context.Context, string) (cacheResult, error) {
	client := redis.NewClient(&redis.Options{Addr: "cache:6379", Protocol: 2, ContextTimeoutEnabled: true, MaxRetries: -1, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	store, err := persistence.New(client, persistence.Options{})
	if err != nil {
		panic(err)
	}
	manager, err := idempotency.New(store, idempotency.Options{KeyPrefix: "cache-orders", EventKeyJMESPath: "id", PayloadValidationJMESPath: "amount"})
	if err != nil {
		panic(err)
	}
	bridge, err := idempotency.New(store, idempotency.Options{KeyPrefix: "cache-interop", EventKeyJMESPath: "id"})
	if err != nil {
		panic(err)
	}
	var warmCalls atomic.Int32
	return func(ctx context.Context, id string) (cacheResult, error) {
		result := cacheResult{}
		warm, err := idempotency.Execute(ctx, manager, order{ID: "warm", Amount: 1}, func(context.Context) (int32, error) { return warmCalls.Add(1), nil })
		if err != nil {
			return result, err
		}
		result.WarmResponse = warm
		result.WarmExecutions = warmCalls.Load()
		_, err = idempotency.Execute(ctx, manager, order{ID: "warm", Amount: 2}, func(context.Context) (int32, error) { return 99, nil })
		var validation *idempotency.ValidationError
		if !errors.As(err, &validation) {
			return result, errors.New("cache validation did not reject changed payload")
		}
		result.ValidationRejected = true
		var executions, contended atomic.Int32
		started, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		payload := order{ID: id, Amount: 1}
		go func() {
			_, err := idempotency.Execute(ctx, manager, payload, func(ctx context.Context) (int, error) {
				executions.Add(1)
				close(started)
				select {
				case <-release:
					return 1, nil
				case <-ctx.Done():
					return 0, ctx.Err()
				}
			})
			done <- err
		}()
		select {
		case <-started:
		case err := <-done:
			return result, err
		case <-ctx.Done():
			return result, ctx.Err()
		}
		var wg sync.WaitGroup
		problems := make(chan error, 32)
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := idempotency.Execute(ctx, manager, payload, func(context.Context) (int, error) { executions.Add(1); return 2, nil })
				var progress *idempotency.AlreadyInProgressError
				if errors.As(err, &progress) {
					contended.Add(1)
				} else {
					problems <- errors.New("unexpected cache acquisition result")
				}
			}()
		}
		wg.Wait()
		close(release)
		close(problems)
		leaderErr := <-done
		if leaderErr != nil {
			return result, leaderErr
		}
		for err := range problems {
			return result, err
		}
		result.ConcurrentExecutions = executions.Load()
		result.ContendedCalls = contended.Load()
		orphan := order{ID: id + "-orphan", Amount: 1}
		key, hash, _, err := manager.Key(orphan)
		if err != nil {
			return result, err
		}
		encoded, _ := json.Marshal(idempotency.Record{Status: idempotency.InProgress, Expiration: time.Now().Unix() + 60, InProgressExpiration: 1, Validation: hash})
		if err = client.Set(ctx, key, encoded, time.Minute).Err(); err != nil {
			return result, err
		}
		recovered, err := idempotency.Execute(ctx, manager, orphan, func(context.Context) (bool, error) { return true, nil })
		if err != nil {
			return result, err
		}
		result.Recovered = recovered
		result.TypeScriptResponse, err = idempotency.Execute(ctx, bridge, order{ID: "typescript"}, func(context.Context) (map[string]any, error) {
			return nil, errors.New("TypeScript record was not replayed")
		})
		if err != nil {
			return result, err
		}
		_, err = idempotency.Execute(ctx, bridge, order{ID: "go"}, func(context.Context) (map[string]any, error) {
			return map[string]any{"owner": "go", "nested": []any{1, nil, true}}, nil
		})
		return result, err
	}
}
