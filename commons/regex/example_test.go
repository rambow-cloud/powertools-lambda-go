package regex_test

import (
	"fmt"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons/regex"
)

func ExampleRegexp_Replace() {
	pattern, err := regex.Compile(`\d`, "g", regex.Options{MatchTimeout: time.Second})
	if err != nil {
		panic(err)
	}
	masked, err := pattern.Replace("order-123", "*")
	fmt.Println(masked, err)
	// Output: order-*** <nil>
}
