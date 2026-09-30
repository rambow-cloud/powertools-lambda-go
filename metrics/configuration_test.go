package metrics

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type referenceConfigService struct {
	Namespace string `json:"getNamespace"`
	Service   string `json:"getServiceName"`
	Fail      string
	Mutate    map[string]*string
	calls     *[]string
	apply     func(map[string]*string)
}

func (c *referenceConfigService) get(method, value string) (string, error) {
	*c.calls = append(*c.calls, method)
	if c.Fail == method {
		return "", fmt.Errorf("configuration failure: %s", method)
	}
	c.apply(c.Mutate)
	return value, nil
}
func (c *referenceConfigService) GetNamespace() (string, error) {
	return c.get("getNamespace", c.Namespace)
}
func (c *referenceConfigService) GetServiceName() (string, error) {
	return c.get("getServiceName", c.Service)
}

type configInput struct {
	OptionSingle       bool
	Namespace, Service *string
	Env, After         map[string]*string
	Custom             *referenceConfigService
	Defaults           Dimensions
	Single, Clear      bool
}
type configResult struct {
	Calls, Warnings               []string
	Emitted                       []map[string]any
	Error                         *string
	Stage                         string
	Parent, Child                 map[string]any
	ParentDisabled, ChildDisabled *bool
}

func TestConfigurationReference(t *testing.T) {
	data, err := os.ReadFile("testdata/config-v2.35.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Now   int64
		Cases []struct {
			Input  configInput
			Result configResult
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 532 {
		t.Fatalf("case count: %d", len(fixture.Cases))
	}
	for index, item := range fixture.Cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			got := runConfigReference(t, fixture.Now, item.Input)
			if !reflect.DeepEqual(got, item.Result) {
				actual, _ := json.MarshalIndent(got, "", "  ")
				want, _ := json.MarshalIndent(item.Result, "", "  ")
				t.Fatalf("got:\n%s\nwant:\n%s", actual, want)
			}
		})
	}
}

func runConfigReference(t *testing.T, now int64, input configInput) (result configResult) {
	t.Helper()
	result = configResult{Calls: []string{}, Warnings: []string{}, Emitted: []map[string]any{}, Stage: "construct"}
	apply := func(values map[string]*string) {
		for key, value := range values {
			t.Setenv(key, "")
			if value == nil {
				if err := os.Unsetenv(key); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv(key, *value)
			}
		}
	}
	apply(map[string]*string{"POWERTOOLS_METRICS_NAMESPACE": nil, "POWERTOOLS_SERVICE_NAME": nil, "POWERTOOLS_METRICS_FUNCTION_NAME": nil, "POWERTOOLS_METRICS_DISABLED": nil, "POWERTOOLS_DEV": nil})
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "on-demand")
	apply(input.Env)
	var output bytes.Buffer
	defer func() { result.Emitted = documents(t, &output) }()
	opts := []Option{WithOutput(&output), WithClock(func() time.Time { return time.UnixMilli(now) }), WithWarningHandler(func(message string) { result.Warnings = append(result.Warnings, message) })}
	opts = append(opts, WithSingleMetric(input.OptionSingle))
	if input.Namespace != nil {
		opts = append(opts, WithNamespace(*input.Namespace))
	}
	if input.Service != nil {
		opts = append(opts, WithServiceName(*input.Service))
	}
	if input.Defaults != nil {
		opts = append(opts, WithDefaultDimensions(input.Defaults))
	}
	if input.Custom != nil {
		service := *input.Custom
		service.calls, service.apply = &result.Calls, apply
		opts = append(opts, WithConfigService(&service))
	}
	failed := func(err error) bool {
		if err == nil {
			return false
		}
		message := err.Error()
		result.Error = &message
		return true
	}
	snapshot := func(m *Metrics, target *map[string]any) error {
		data, err := m.Serialize()
		if err != nil {
			return err
		}
		return json.Unmarshal(data, target)
	}
	m, err := New(opts...)
	if failed(err) {
		return
	}
	disabled := m.Disabled()
	result.ParentDisabled = &disabled
	if failed(m.SetTimestamp(time.UnixMilli(now-3600000))) || failed(m.AddMetric("Parent", Count, 2)) || failed(snapshot(m, &result.Parent)) {
		return
	}
	if input.Single {
		if input.Clear && failed(m.ClearDefaultDimensions()) {
			return
		}
		apply(input.After)
		result.Stage = "single"
		child, err := m.SingleMetric()
		if failed(err) {
			return
		}
		disabled := child.Disabled()
		result.ChildDisabled = &disabled
		if failed(snapshot(child, &result.Child)) || failed(child.AddMetric("Child", Count, 3)) || failed(child.CaptureColdStartMetric("child-fallback")) {
			return
		}
	}
	result.Stage = "flush"
	if failed(m.Flush()) {
		return
	}
	result.Stage = "done"
	return
}

type failingConfigService struct{ failure error }

func (c failingConfigService) GetNamespace() (string, error)   { return "", c.failure }
func (c failingConfigService) GetServiceName() (string, error) { return "", c.failure }

func TestConfigurationErrorsAndSingleMetricOverrides(t *testing.T) {
	t.Setenv("AWS_LAMBDA_INITIALIZATION_TYPE", "on-demand")
	m, output := setup(t, WithDisabled(true), WithRequireMetrics(true))
	child, err := m.SingleMetric()
	if err != nil {
		t.Fatal(err)
	}
	if child.Disabled() {
		t.Fatal("single metric inherited explicit disabling")
	}
	if _, err := child.Serialize(); err != nil {
		t.Fatalf("single metric inherited empty policy: %v", err)
	}
	if err := child.AddMetric("Independent", Count, 1); err != nil {
		t.Fatal(err)
	}
	if len(documents(t, output)) != 1 {
		t.Fatal("single metric did not use the shared output")
	}
	failure := errors.New("custom configuration unavailable")
	if _, err := New(WithConfigService(failingConfigService{failure})); err != failure {
		t.Fatalf("custom error identity: %v", err)
	}
	t.Setenv("POWERTOOLS_METRICS_DISABLED", "invalid")
	_, err = New(WithDisabled(false), WithConfigService(failingConfigService{failure}))
	var environment *commons.EnvironmentError
	if !errors.As(err, &environment) || environment.Key != "POWERTOOLS_METRICS_DISABLED" {
		t.Fatalf("environment precedence: %v", err)
	}
	if _, err := m.SingleMetric(); !errors.As(err, &environment) {
		t.Fatalf("single environment validation: %v", err)
	}
	if err := m.CaptureColdStartMetric(); !errors.As(err, &environment) {
		t.Fatalf("cold-start construction error: %v", err)
	}
	t.Setenv("POWERTOOLS_METRICS_DISABLED", "false")
	if err := m.CaptureColdStartMetric(); err != nil {
		t.Fatal(err)
	}
	if len(documents(t, output)) != 1 {
		t.Fatal("failed construction did not consume the cold-start decision")
	}
}
