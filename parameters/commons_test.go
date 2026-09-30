package parameters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
)

func TestSharedMaxAgeValidationBeforeCacheLookup(t *testing.T) {
	cache := parameters.NewCache(nil)
	calls := 0
	fetch := func(context.Context) (any, error) { calls++; return "value", nil }
	t.Setenv("POWERTOOLS_PARAMETERS_MAX_AGE", "0x10")
	_, err := cache.Get(context.Background(), "key", parameters.Options{}, fetch)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("POWERTOOLS_PARAMETERS_MAX_AGE", "invalid")
	_, err = cache.Get(context.Background(), "key", parameters.Options{}, fetch)
	var env *commons.EnvironmentError
	if !errors.As(err, &env) || calls != 1 {
		t.Fatalf("cache bypassed configuration validation: %v calls=%d", err, calls)
	}
	_, err = cache.Get(context.Background(), "key", parameters.Options{MaxAge: parameters.Age(0)}, fetch)
	if err != nil || calls != 1 {
		t.Fatal("explicit lifetime should bypass environment and retain valid entry")
	}
}
