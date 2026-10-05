package parser_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func TestCloudFormationResourceIdentifier(t *testing.T) {
	for _, test := range []struct {
		name   string
		schema parser.Schema[any]
	}{
		{"CloudFormationCustomResourceUpdateSchema", schemas.CloudFormationCustomResourceUpdateSchema},
		{"CloudFormationCustomResourceDeleteSchema", schemas.CloudFormationCustomResourceDeleteSchema},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := serviceFixture(t, "services", test.name)
			input["PhysicalResourceId"] = "existing-resource-123"
			handler := parser.WrapHandler[any, any, any](test.schema, func(_ context.Context, parsed any) (any, error) { return parsed, nil })
			output, err := handler(context.Background(), input)
			if err != nil || !reflect.DeepEqual(jsonValue(output), input) {
				t.Fatalf("resource identifier lost before handler: %v / %v", output, err)
			}
			for _, invalid := range []struct {
				value  any
				absent bool
			}{{nil, true}, {nil, false}, {float64(123), false}, {true, false}} {
				if invalid.absent {
					delete(input, "PhysicalResourceId")
				} else {
					input["PhysicalResourceId"] = invalid.value
				}
				result, err := parser.SafeParse(context.Background(), input, test.schema)
				if err != nil || result.Success || result.Error == nil || len(result.Error.Issues) != 1 || !reflect.DeepEqual(result.Error.Issues[0].Path, []any{"PhysicalResourceId"}) {
					t.Fatalf("invalid identifier not rejected at its field: %+v / %v", result, err)
				}
			}
		})
	}
}

func TestCloudFormationCreateDoesNotRequireResourceIdentifier(t *testing.T) {
	input := serviceFixture(t, "services", "CloudFormationCustomResourceCreateSchema")
	delete(input, "PhysicalResourceId")
	output, err := parser.Parse(context.Background(), input, schemas.CloudFormationCustomResourceCreateSchema)
	if err != nil || !reflect.DeepEqual(jsonValue(output), input) {
		t.Fatalf("create requires an existing resource: %v / %v", output, err)
	}
}
