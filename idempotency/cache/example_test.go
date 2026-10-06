package cache_test

import (
	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	"github.com/rambow-cloud/powertools-lambda-go/idempotency/cache"
	"github.com/redis/go-redis/v9"
)

// This example configures a shared store. Actual operations require Redis or Valkey.
func ExampleNew() {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()
	store, err := cache.New(client, cache.Options{})
	if err != nil {
		panic(err)
	}
	manager, err := idempotency.New(store, idempotency.Options{
		KeyPrefix: "orders", EventKeyJMESPath: "orderId",
	})
	if err != nil {
		panic(err)
	}
	_ = manager // Reuse the manager and wrap business operations during invocations.
}
