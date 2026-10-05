package parser_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func serviceFixture(t *testing.T, file, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile("testdata/" + file + "-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name  string
			Input any
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, item := range fixture.Cases {
		if item.Name == name+"-valid" {
			return item.Input.(map[string]any)
		}
	}
	t.Fatalf("missing fixture: %s", name)
	return nil
}

func sesParts(input map[string]any) (map[string]any, map[string]any) {
	ses := input["Records"].([]any)[0].(map[string]any)["ses"].(map[string]any)
	return ses["receipt"].(map[string]any), ses["mail"].(map[string]any)["commonHeaders"].(map[string]any)
}

func TestSESReceiptVariants(t *testing.T) {
	cases := []struct {
		name   string
		change func(map[string]any, map[string]any)
	}{
		{"no-dmarc-policy", func(receipt, _ map[string]any) { delete(receipt, "dmarcPolicy") }},
		{"request-response", func(receipt, _ map[string]any) {
			receipt["action"].(map[string]any)["invocationType"] = "RequestResponse"
		}},
		{"no-common-members", func(_, headers map[string]any) { clear(headers) }},
		{"reply-to", func(_, headers map[string]any) { headers["replyTo"] = []any{"reply@example.test"} }},
	}
	for _, header := range []string{"from", "to", "returnPath", "messageId", "date", "subject"} {
		cases = append(cases, struct {
			name   string
			change func(map[string]any, map[string]any)
		}{"no-" + header, func(_, headers map[string]any) { delete(headers, header) }})
	}
	for _, policy := range []string{"none", "quarantine", "reject"} {
		cases = append(cases, struct {
			name   string
			change func(map[string]any, map[string]any)
		}{"policy-" + policy, func(receipt, _ map[string]any) {
			receipt["dmarcVerdict"].(map[string]any)["status"] = "FAIL"
			receipt["dmarcPolicy"] = policy
		}})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := serviceFixture(t, "services", "SesSchema")
			receipt, headers := sesParts(input)
			test.change(receipt, headers)
			output, err := parser.Parse(context.Background(), input, schemas.SesSchema)
			if err != nil || !reflect.DeepEqual(jsonValue(output), input) {
				t.Fatalf("SES fields lost or rejected: output=%v error=%v", output, err)
			}
		})
	}
}

func TestSESOptionalFieldsStillValidate(t *testing.T) {
	for _, field := range []string{"dmarcPolicy", "invocationType", "from", "to", "cc", "bcc", "sender", "replyTo", "returnPath", "messageId", "date", "subject"} {
		t.Run(field, func(t *testing.T) {
			input := serviceFixture(t, "services", "SesSchema")
			receipt, headers := sesParts(input)
			switch field {
			case "dmarcPolicy":
				receipt[field] = "invalid"
			case "invocationType":
				receipt["action"].(map[string]any)[field] = "invalid"
			default:
				headers[field] = false
			}
			result, err := parser.SafeParse(context.Background(), input, schemas.SesSchema)
			if err != nil || result.Success || result.Error == nil {
				t.Fatalf("invalid present field accepted: %+v error=%v", result, err)
			}
		})
	}
}

func TestSESHandlerReceivesConditionalFields(t *testing.T) {
	input := serviceFixture(t, "services", "SesSchema")
	receipt, headers := sesParts(input)
	delete(receipt, "dmarcPolicy")
	clear(headers)
	headers["replyTo"] = []any{"reply@example.test"}
	receipt["action"].(map[string]any)["invocationType"] = "RequestResponse"
	handler := parser.WrapHandler[any, any, any](schemas.SesSchema, func(_ context.Context, parsed any) (any, error) { return parsed, nil })
	output, err := handler(context.Background(), input)
	if err != nil || !reflect.DeepEqual(jsonValue(output), input) {
		t.Fatalf("valid SES event did not reach the handler unchanged: %v / %v", output, err)
	}
}
