package parameters_test

import (
	"context"
	"fmt"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

func ExampleCache_Get() {
	cache := parameters.NewCache(nil)
	options := parameters.Options{MaxAge: parameters.Age(time.Minute)}
	fetches := 0
	fetch := func(context.Context) (any, error) {
		fetches++
		return "enabled", nil
	}
	for range 2 {
		value, err := cache.Get(context.Background(), "feature", options, fetch)
		fmt.Println(value, err)
	}
	fmt.Println("fetches:", fetches)
	// Output:
	// enabled <nil>
	// enabled <nil>
	// fetches: 1
}
