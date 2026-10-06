package awssdk_test

import (
	"fmt"

	"github.com/aws/smithy-go/middleware"
	"github.com/rambow-cloud/powertools-lambda-go/commons/awssdk"
)

func ExampleUserAgent() {
	stack := middleware.NewStack("example", func() any { return nil })
	err := awssdk.UserAgent("parameters")(stack)
	_, installed := stack.Build.Get("PowertoolsUserAgent")
	fmt.Println(installed, err)
	// Output: true <nil>
}
