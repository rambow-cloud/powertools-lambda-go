// Command query decodes an SQS envelope and extracts a correlation value offline.
package main

import (
	"context"
	"os"

	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	if err := run(appLog); err != nil {
		if logErr := appLog.Error("Application failed", err); logErr != nil {
			panic(logErr)
		}
		os.Exit(1)
	}
}

func run(appLog *logger.Logger) error {
	event := map[string]any{"Records": []any{map[string]any{"body": `{"orderId":"order-1"}`}}}
	payloads, err := jmespath.ExtractDataFromEnvelope(event, jmespath.SQS)
	if err != nil {
		return err
	}
	if err := appLog.Info("Envelope decoded", logger.Fields{"payloads": payloads}); err != nil {
		return err
	}
	handler := logger.WrapHandler(appLog, func(ctx context.Context, _ any) (string, error) {
		return "ok", appLog.WithContext(ctx).Info("order received")
	}, logger.HandlerOptions{
		CorrelationExtractor: jmespath.MustCompile("Records[0].powertools_json(body).orderId", jmespath.WithPowertoolsFunctions()),
	})
	if _, err = handler(context.Background(), event); err != nil {
		return err
	}
	return nil
}
