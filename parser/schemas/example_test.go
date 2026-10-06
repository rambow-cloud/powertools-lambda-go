package schemas_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func Example() {
	// SQS deliveries require at least one record. SafeParse retains validation
	// details so the caller can reject an invalid envelope before business code.
	event := map[string]any{"Records": []any{}}
	result, err := parser.SafeParse(context.Background(), event, schemas.SqsSchema)
	fmt.Println(result.Success, result.Error != nil, err)
	// Output: false true <nil>
}
