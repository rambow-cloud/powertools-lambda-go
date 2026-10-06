package awssdk

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
)

type userAgent struct{ feature, environment string }

func (userAgent) ID() string { return "PowertoolsUserAgent" }
func (m userAgent) HandleBuild(ctx context.Context, input middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
	if request, ok := input.Request.(*smithyhttp.Request); ok {
		existing := request.Header.Get("User-Agent")
		marker := fmt.Sprintf("PT/%s/%s PTEnv/%s", m.feature, commons.Version, m.environment)
		if strings.Contains(existing, "PT/NO-OP") {
			existing = strings.Replace(existing, "PT/NO-OP", "PT/"+m.feature, 1)
		} else if !strings.Contains(existing, "PT/") {
			existing = strings.TrimSpace(existing + " " + marker)
		}
		request.Header.Set("User-Agent", existing)
	}
	return next.HandleBuild(ctx, input)
}

// UserAgent returns an idempotent per-request stack option. It does not mutate
// environment variables or a shared client; the first installed feature wins.
func UserAgent(feature string) func(*middleware.Stack) error {
	environment := os.Getenv("AWS_EXECUTION_ENV")
	if environment == "" {
		environment = "NA"
	}
	return func(stack *middleware.Stack) error {
		if _, ok := stack.Build.Get("PowertoolsUserAgent"); ok {
			return nil
		}
		return stack.Build.Add(userAgent{feature: feature, environment: environment}, middleware.After)
	}
}
