package datamasking_test

import (
	"context"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/datamasking"
)

func ExampleMasker_Erase() {
	masker := datamasking.New(datamasking.Config{})
	input := map[string]any{"email": "user@example.com", "order": "order-123"}
	value, err := masker.Erase(context.Background(), input,
		datamasking.EraseOptions{Fields: []string{"email"}})
	if err != nil {
		panic(err)
	}
	masked := value.(map[string]any)
	fmt.Println(masked["email"], masked["order"])
	fmt.Println(input["email"])
	// Output:
	// ***** order-123
	// user@example.com
}
