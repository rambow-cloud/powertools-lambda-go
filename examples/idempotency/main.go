// Package main demonstrates durable idempotency for a typed Lambda handler.
package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	persistence "github.com/rambow-cloud/powertools-lambda-go/idempotency/dynamodb"
)

type Order struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}
type Result struct {
	OrderID string `json:"order_id"`
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	store, err := persistence.New(dynamodb.NewFromConfig(cfg), persistence.Options{TableName: os.Getenv("IDEMPOTENCY_TABLE")})
	if err != nil {
		log.Fatal(err)
	}
	manager, err := idempotency.New(store, idempotency.Options{KeyPrefix: "process-order", EventKeyJMESPath: "id", PayloadValidationJMESPath: "amount", ThrowOnNoKey: true})
	if err != nil {
		log.Fatal(err)
	}
	lambda.Start(idempotency.WrapHandler(manager, func(ctx context.Context, event Order) (Result, error) {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		return Result{OrderID: event.ID}, nil
	}))
}
