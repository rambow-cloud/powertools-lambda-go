package metrics

import (
	"log"
	"os"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

// ConfigService supplies the two custom settings used by the reference constructor.
// Getters run only when the corresponding explicit option is empty. Errors propagate
// unchanged. The reference declares a function-name getter but does not call it.
type ConfigService interface {
	GetNamespace() (string, error)
	GetServiceName() (string, error)
}

// WithConfigService supplies constructor fallbacks before environment settings.
// The service is called during construction, outside all metric storage locks.
func WithConfigService(service ConfigService) Option {
	return func(c *config) { c.configService = service }
}

func defaults() (config, error) {
	namespace, _ := commons.StringEnv("POWERTOOLS_METRICS_NAMESPACE", "")
	function, _ := commons.StringEnv("POWERTOOLS_METRICS_FUNCTION_NAME", "")
	service := commons.ServiceName()
	disabled, err := commons.BoolEnv("POWERTOOLS_METRICS_DISABLED", true, false)
	if err != nil {
		return config{}, err
	}
	if _, present := os.LookupEnv("POWERTOOLS_METRICS_DISABLED"); !present {
		disabled = commons.IsDevMode()
	}
	logger := log.New(os.Stderr, "", 0)
	return config{envNamespace: namespace, envService: service, function: function, writer: os.Stdout, clock: time.Now, disabled: disabled, onError: func(error) {}, onWarning: func(message string) { logger.Println(message) }}, nil
}

func (c *config) resolve() error {
	var err error
	if c.namespace == "" && c.configService != nil {
		c.namespace, err = c.configService.GetNamespace()
		if err != nil {
			return err
		}
	}
	if c.namespace == "" {
		c.namespace = c.envNamespace
	}
	if c.service == "" && c.configService != nil {
		c.service, err = c.configService.GetServiceName()
		if err != nil {
			return err
		}
	}
	if c.service == "" {
		c.service = c.envService
	}
	if c.service == "" {
		c.service = "service_undefined"
	}
	return nil
}
