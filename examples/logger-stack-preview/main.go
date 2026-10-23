// This Preview is not included in v1.2.0.
package main

import (
	"errors"

	"github.com/rambow-cloud/powertools-lambda-go/logger"
)

func loadOrder() error {
	return logger.WithStack(errors.New("order not found"))
}

func main() {
	appLog := logger.New(logger.WithServiceName("orders"))
	if logErr := appLog.Error("Cannot load order", loadOrder()); logErr != nil {
		panic(logErr)
	}
	if logErr := appLog.Error("Payment failed", errors.New("card declined")); logErr != nil {
		panic(logErr)
	}
}
