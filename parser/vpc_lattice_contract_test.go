package parser_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func latticeV2Event(t *testing.T) map[string]any {
	t.Helper()
	input := serviceFixture(t, "http", "VpcLatticeV2Schema")
	input["headers"] = map[string]any{"content-type": []any{"application/json"}, "x-tag": []any{"first", "second"}}
	input["queryStringParameters"] = map[string]any{"tag": []any{"a", "b"}, "single": []any{"value"}}
	input["requestId"] = "request-123"
	identity := input["requestContext"].(map[string]any)["identity"].(map[string]any)
	for legacy, canonical := range map[string]string{"principalOrgId": "principalOrgID", "X509SubjectCn": "x509SubjectCn", "X509IssuerOu": "x509IssuerOu", "X509SanNameCn": "x509SanNameCn"} {
		identity[canonical] = identity[legacy]
		delete(identity, legacy)
	}
	return input
}

func TestVpcLatticeV2HandlerPreservesServiceFields(t *testing.T) {
	for _, optional := range []bool{true, false} {
		input := latticeV2Event(t)
		if !optional {
			delete(input, "queryStringParameters")
			delete(input, "requestId")
		}
		handler := parser.WrapHandler[any, any, any](schemas.VpcLatticeV2Schema, func(_ context.Context, value any) (any, error) { return value, nil })
		output, err := handler(context.Background(), input)
		if err != nil || !reflect.DeepEqual(jsonValue(output), input) {
			t.Fatalf("V2 service event lost or rejected: %v / %v", output, err)
		}
	}
}

func TestVpcLatticeV2EnvelopeWithRepeatedValues(t *testing.T) {
	input := latticeV2Event(t)
	payload := parser.JSONStringified(parser.Object(parser.Field{Name: "id", Schema: parser.String()}, parser.Field{Name: "amount", Schema: parser.Number()}))
	handler := parser.WrapHandler[any, any, any](envelopes.VpcLatticeV2(payload), func(_ context.Context, value any) (any, error) { return value, nil })
	output, err := handler(context.Background(), input)
	if err != nil || !reflect.DeepEqual(jsonValue(output), map[string]any{"id": "http", "amount": float64(1)}) {
		t.Fatalf("array metadata prevented payload delivery: %v / %v", output, err)
	}
}

func TestVpcLatticeV2InvalidPresentFields(t *testing.T) {
	for _, test := range []struct {
		field string
		value any
		path  []any
	}{
		{"headers", map[string]any{"x": "scalar"}, []any{"headers", "x"}},
		{"queryStringParameters", map[string]any{"x": "scalar"}, []any{"queryStringParameters", "x"}},
		{"headers", map[string]any{"x": []any{"valid", false}}, []any{"headers", "x", 1}},
		{"queryStringParameters", map[string]any{"x": []any{nil}}, []any{"queryStringParameters", "x", 0}},
		{"requestId", false, []any{"requestId"}},
		{"requestId", nil, []any{"requestId"}},
		{"principalOrgID", 1, []any{"requestContext", "identity", "principalOrgID"}},
		{"x509SubjectCn", nil, []any{"requestContext", "identity", "x509SubjectCn"}},
		{"x509IssuerOu", false, []any{"requestContext", "identity", "x509IssuerOu"}},
		{"x509SanNameCn", []any{}, []any{"requestContext", "identity", "x509SanNameCn"}},
	} {
		input := latticeV2Event(t)
		if len(test.path) == 3 && test.path[0] == "requestContext" {
			input["requestContext"].(map[string]any)["identity"].(map[string]any)[test.field] = test.value
		} else {
			input[test.field] = test.value
		}
		result, err := parser.SafeParse(context.Background(), input, schemas.VpcLatticeV2Schema)
		if err != nil || result.Success || result.Error == nil || len(result.Error.Issues) != 1 || !reflect.DeepEqual(result.Error.Issues[0].Path, test.path) {
			t.Fatalf("invalid %s not rejected at field: %+v / %v", test.field, result, err)
		}
	}
}

func TestVpcLatticeLegacyShapesRemainValid(t *testing.T) {
	for _, test := range []struct {
		name   string
		schema parser.Schema[any]
	}{{"VpcLatticeSchema", schemas.VpcLatticeSchema}, {"VpcLatticeV2Schema", schemas.VpcLatticeV2Schema}} {
		input := serviceFixture(t, "http", test.name)
		if test.name == "VpcLatticeSchema" {
			input["headers"] = map[string]any{"x": "scalar"}
			input["query_string_parameters"] = map[string]any{"tag": "scalar"}
		}
		output, err := parser.Parse(context.Background(), input, test.schema)
		if err != nil || !reflect.DeepEqual(jsonValue(output), input) {
			t.Fatalf("legacy %s lost or rejected: %v / %v", test.name, output, err)
		}
	}
}
