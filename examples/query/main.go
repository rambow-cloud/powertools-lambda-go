// Command query decodes an SQS envelope and extracts a correlation value offline.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func main() {
	event := map[string]any{"Records": []any{map[string]any{"body": `{"orderId":"order-1"}`}}}
	payloads, err := jmespath.ExtractDataFromEnvelope(event, jmespath.SQS)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(payloads)
	l := logger.New()
	handler := logger.WrapHandler(l, func(ctx context.Context, _ any) (string, error) {
		return "ok", l.WithContext(ctx).Info("order received")
	}, logger.HandlerOptions{
		CorrelationExtractor: jmespath.MustCompile("Records[0].powertools_json(body).orderId", jmespath.WithPowertoolsFunctions()),
	})
	if _, err = handler(context.Background(), event); err != nil {
		log.Fatal(err)
	}
}
