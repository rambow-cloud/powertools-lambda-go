package idempotency_test

import (
	"fmt"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
)

func ExampleRecord_CurrentStatus() {
	now := time.Unix(1_700_000_000, 0)
	record := idempotency.Record{Key: "order-123", Status: idempotency.Completed,
		Expiration: now.Add(time.Hour).Unix()}
	status, err := record.CurrentStatus(now)
	fmt.Println(status, err)
	status, err = record.CurrentStatus(now.Add(2 * time.Hour))
	fmt.Println(status, err)
	// Output:
	// COMPLETED <nil>
	// EXPIRED <nil>
}
