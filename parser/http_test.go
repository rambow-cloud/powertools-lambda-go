package parser_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/parser"
	"github.com/rambow-cloud/powertools-lambda-go/parser/envelopes"
	"github.com/rambow-cloud/powertools-lambda-go/parser/schemas"
)

func latticeEvent(body any) map[string]any {
	return map[string]any{"method": "POST", "raw_path": "/", "body": body, "is_base64_encoded": false, "headers": map[string]any{}, "query_string_parameters": map[string]any{}}
}

func TestObjectEnvelopeModesIsolationAndErrors(t *testing.T) {
	ctx := context.Background()
	event := latticeEvent("input")
	envelope := envelopes.VpcLattice[any](modeSchema{})
	result, err := parser.SafeParse(ctx, event, envelope)
	if err != nil || result.Error == nil || len(result.Error.Issues) != 2 {
		t.Fatalf("safe mode: %+v %v", result, err)
	}
	if !reflect.DeepEqual(result.Error.Issues[1].Path, []any{"body"}) {
		t.Fatal("payload issue path missing")
	}
	_, err = parser.Parse(ctx, event, envelope)
	var failure *parser.ParseError
	if !errors.As(err, &failure) || len(failure.Issues) != 1 {
		t.Fatalf("ordinary mode: %v", err)
	}
	if _, err := parser.Parse(ctx, event, schemas.VpcLatticeSchema); err != nil {
		t.Fatalf("envelope mutated shared schema: %v", err)
	}
	sentinel := errors.New("validation callback failed")
	failing := envelopes.VpcLattice(parser.SchemaFunc[any](func(context.Context, any) (any, []parser.Issue, error) { return nil, nil, sentinel }))
	if _, err := parser.SafeParse(ctx, event, failing); err != sentinel {
		t.Fatalf("operational error changed: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := parser.Parse(canceled, event, envelope); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost")
	}
	if _, err := parser.Parse(ctx, event, envelopes.VpcLattice[any](nil)); err == nil {
		t.Fatal("nil payload schema accepted")
	}
}

func TestObjectEnvelopeAbsentUnknownAndConcurrentReuse(t *testing.T) {
	ctx := context.Background()
	event := latticeEvent(nil)
	delete(event, "body")
	value, err := parser.Parse(ctx, event, envelopes.VpcLattice(parser.Unknown()))
	if err != nil || value != nil {
		t.Fatalf("missing body leaked internal absence: %#v %v", value, err)
	}
	eventBridge := map[string]any{"version": "0", "id": "id", "source": "source", "account": "account", "time": "2026-09-15T00:00:00Z", "region": "ap-east-1", "resources": []any{}, "detail-type": "order"}
	value, err = parser.Parse(ctx, eventBridge, envelopes.EventBridge(parser.Unknown()))
	if err != nil || value != nil {
		t.Fatalf("missing detail leaked internal absence: %#v %v", value, err)
	}
	envelope := envelopes.VpcLattice(parser.Typed[struct {
		ID string `json:"id"`
	}](parser.Object(parser.Field{Name: "id", Schema: parser.String()})))
	var workers sync.WaitGroup
	for range 100 {
		workers.Go(func() {
			result, err := parser.Parse(ctx, latticeEvent(map[string]any{"id": "value"}), envelope)
			if err != nil || result.ID != "value" {
				t.Errorf("concurrent result: %+v %v", result, err)
			}
		})
	}
	workers.Wait()
}
