package commons_test

import (
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

func ExampleNewLRUCache() {
	cache := commons.NewLRUCache[string, string](2)
	cache.Add("region", "ap-east-1")
	region, found := cache.Get("region")
	fmt.Println(region, found)
	// Output: ap-east-1 true
}
