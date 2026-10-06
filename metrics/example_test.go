package metrics_test

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/rambow-cloud/powertools-lambda-go/metrics"
)

func ExampleMetrics_Flush() {
	var output bytes.Buffer
	m, err := metrics.New(metrics.WithNamespace("Example/Orders"),
		metrics.WithServiceName("orders"), metrics.WithOutput(&output), metrics.WithDisabled(false))
	if err != nil {
		panic(err)
	}
	if err := m.AddMetric("OrdersAccepted", metrics.Count, 1); err != nil {
		panic(err)
	}
	if err := m.Flush(); err != nil {
		panic(err)
	}
	var document map[string]any
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		panic(err)
	}
	fmt.Println(document["OrdersAccepted"])
	// Output: 1
}
