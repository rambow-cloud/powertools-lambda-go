package parameters_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	secretssdk "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	ssmsdk "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/rambow-cloud/powertools-lambda-go/parameters"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/secrets"
	"github.com/rambow-cloud/powertools-lambda-go/parameters/ssm"
)

type variantSSM struct{ singles, batches, paths int }

func (f *variantSSM) GetParameter(_ context.Context, in *ssmsdk.GetParameterInput, _ ...func(*ssmsdk.Options)) (*ssmsdk.GetParameterOutput, error) {
	f.singles++
	return &ssmsdk.GetParameterOutput{Parameter: &types.Parameter{Value: aws.String(fmt.Sprint(aws.ToBool(in.WithDecryption)))}}, nil
}
func (f *variantSSM) GetParameters(_ context.Context, in *ssmsdk.GetParametersInput, _ ...func(*ssmsdk.Options)) (*ssmsdk.GetParametersOutput, error) {
	f.batches++
	out := &ssmsdk.GetParametersOutput{}
	for _, name := range in.Names {
		out.Parameters = append(out.Parameters, types.Parameter{Name: aws.String(name), Value: aws.String(fmt.Sprint(aws.ToBool(in.WithDecryption)))})
	}
	return out, nil
}
func (f *variantSSM) GetParametersByPath(_ context.Context, in *ssmsdk.GetParametersByPathInput, _ ...func(*ssmsdk.Options)) (*ssmsdk.GetParametersByPathOutput, error) {
	f.paths++
	value, _ := json.Marshal(in)
	return &ssmsdk.GetParametersByPathOutput{Parameters: []types.Parameter{{Name: aws.String(aws.ToString(in.Path) + "/item"), Value: aws.String(string(value))}}}, nil
}
func (*variantSSM) PutParameter(context.Context, *ssmsdk.PutParameterInput, ...func(*ssmsdk.Options)) (*ssmsdk.PutParameterOutput, error) {
	return nil, errors.New("unused")
}

func TestSSMEffectiveCacheOptions(t *testing.T) {
	t.Setenv("POWERTOOLS_PARAMETERS_SSM_DECRYPT", "false")
	f := &variantSSM{}
	p := ssm.New(f)
	ctx := context.Background()
	get := func(o ssm.GetOptions, want string, calls int) {
		t.Helper()
		v, e := p.Get(ctx, "key", o)
		if e != nil || v != want || f.singles != calls {
			t.Fatalf("get=%v/%v calls=%d want %s/%d", v, e, f.singles, want, calls)
		}
	}
	get(ssm.GetOptions{}, "false", 1)
	get(ssm.GetOptions{Decrypt: aws.Bool(true)}, "true", 2)
	get(ssm.GetOptions{SDKOptions: &ssmsdk.GetParameterInput{WithDecryption: aws.Bool(false)}}, "false", 2)
	get(ssm.GetOptions{Decrypt: aws.Bool(true), SDKOptions: &ssmsdk.GetParameterInput{WithDecryption: aws.Bool(false)}}, "true", 2)
	t.Setenv("POWERTOOLS_PARAMETERS_SSM_DECRYPT", "true")
	get(ssm.GetOptions{}, "true", 2)
	get(ssm.GetOptions{Options: parameters.Options{MaxAge: parameters.Age(0)}}, "true", 3)
	get(ssm.GetOptions{}, "true", 3)
	get(ssm.GetOptions{Options: parameters.Options{ForceFetch: true}}, "true", 4)
	t.Setenv("POWERTOOLS_PARAMETERS_SSM_DECRYPT", "invalid")
	if _, e := p.Get(ctx, "key", ssm.GetOptions{}); e == nil || f.singles != 4 {
		t.Fatalf("cached read ignored invalid decrypt environment: %v calls=%d", e, f.singles)
	}
	get(ssm.GetOptions{Decrypt: aws.Bool(false)}, "false", 4)
	p.ClearCache()
	get(ssm.GetOptions{Decrypt: aws.Bool(false)}, "false", 5)
}

func TestSSMPathCacheOptions(t *testing.T) {
	t.Setenv("POWERTOOLS_PARAMETERS_SSM_DECRYPT", "false")
	f := &variantSSM{}
	p := ssm.New(f)
	ctx := context.Background()
	inputs := []*ssmsdk.GetParametersByPathInput{
		{Recursive: aws.Bool(false), WithDecryption: aws.Bool(false)},
		{Recursive: aws.Bool(true), WithDecryption: aws.Bool(false)},
		{Recursive: aws.Bool(true), WithDecryption: aws.Bool(true)},
		{Recursive: aws.Bool(true), WithDecryption: aws.Bool(true), ParameterFilters: []types.ParameterStringFilter{{Key: aws.String("Type"), Values: []string{"SecureString"}}}},
		{Recursive: aws.Bool(true), WithDecryption: aws.Bool(true), MaxResults: aws.Int32(2)},
		{Recursive: aws.Bool(true), WithDecryption: aws.Bool(true), NextToken: aws.String("start")},
	}
	for i, in := range inputs {
		before, _ := json.Marshal(in)
		v, e := p.GetMultiple(ctx, "/app", ssm.MultipleOptions{SDKOptions: in})
		effective := *in
		effective.Path = aws.String("/app")
		want, _ := json.Marshal(effective)
		if e != nil || v["item"] != string(want) || f.paths != i+1 {
			t.Fatalf("variant %d: %v/%v calls=%d want %s", i, v, e, f.paths, want)
		}
		after, _ := json.Marshal(in)
		if string(before) != string(after) {
			t.Fatal("caller SDK options mutated")
		}
	}
	for _, in := range inputs {
		if _, e := p.GetMultiple(ctx, "/app", ssm.MultipleOptions{SDKOptions: in}); e != nil {
			t.Fatal(e)
		}
	}
	if f.paths != len(inputs) {
		t.Fatalf("variants were not cached separately: %d", f.paths)
	}
	// Explicit provider options override SDK values and select the matching cached request.
	v, e := p.GetMultiple(ctx, "/app", ssm.MultipleOptions{Decrypt: aws.Bool(false), Recursive: aws.Bool(false), SDKOptions: inputs[2]})
	if e != nil || f.paths != len(inputs) || !reflect.DeepEqual(v, map[string]any{"item": mustInputJSON(t, inputs[0], "/app")}) {
		t.Fatalf("precedence: %v/%v calls=%d", v, e, f.paths)
	}
}
func mustInputJSON(t *testing.T, in *ssmsdk.GetParametersByPathInput, path string) string {
	t.Helper()
	cp := *in
	cp.Path = aws.String(path)
	b, e := json.Marshal(cp)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

type variantSecrets struct{ calls int }

func (f *variantSecrets) GetSecretValue(_ context.Context, in *secretssdk.GetSecretValueInput, _ ...func(*secretssdk.Options)) (*secretssdk.GetSecretValueOutput, error) {
	f.calls++
	v, _ := json.Marshal(in)
	return &secretssdk.GetSecretValueOutput{SecretString: aws.String(string(v))}, nil
}
func TestSecretsVersionCacheOptions(t *testing.T) {
	f := &variantSecrets{}
	p := secrets.New(f)
	ctx := context.Background()
	inputs := []*secretssdk.GetSecretValueInput{{}, {VersionStage: aws.String("AWSCURRENT")}, {VersionStage: aws.String("AWSPREVIOUS")}, {VersionId: aws.String("version-a")}, {VersionId: aws.String("version-b")}, {VersionStage: aws.String("AWSCURRENT"), VersionId: aws.String("version-a")}, {VersionStage: aws.String("")}}
	for i, in := range inputs {
		before, _ := json.Marshal(in)
		v, e := p.Get(ctx, "secret", secrets.GetOptions{SDKOptions: in})
		effective := *in
		effective.SecretId = aws.String("secret")
		want, _ := json.Marshal(effective)
		after, _ := json.Marshal(in)
		if e != nil || v != string(want) || f.calls != i+1 || string(before) != string(after) {
			t.Fatalf("variant %d: %v/%v calls=%d want %s", i, v, e, f.calls, want)
		}
	}
	for _, in := range inputs {
		if _, e := p.Get(ctx, "secret", secrets.GetOptions{SDKOptions: in}); e != nil {
			t.Fatal(e)
		}
	}
	if f.calls != len(inputs) {
		t.Fatalf("cache not reused: %d", f.calls)
	}
}

func TestSSMBatchCacheIsolationAndZeroAge(t *testing.T) {
	ctx := context.Background()
	f := &variantSSM{}
	p := ssm.New(f)
	if _, e := p.Get(ctx, "key", ssm.GetOptions{Decrypt: aws.Bool(true)}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		v, e := p.GetParametersByName(ctx, map[string]ssm.GetOptions{"key": {}}, ssm.ByNameOptions{Decrypt: aws.Bool(false)})
		if e != nil || v["key"] != "false" || f.batches != 1 {
			t.Fatalf("batch variant %v/%v calls=%d", v, e, f.batches)
		}
	}
	for _, decrypt := range []bool{false, true} {
		v, e := p.Get(ctx, "key", ssm.GetOptions{Decrypt: aws.Bool(decrypt)})
		if e != nil || v != fmt.Sprint(decrypt) || f.singles != 1 {
			t.Fatalf("single/batch isolation: %v/%v calls=%d", v, e, f.singles)
		}
	}
	zero := ssm.ByNameOptions{Options: parameters.Options{MaxAge: parameters.Age(0)}, Decrypt: aws.Bool(false)}
	if _, e := p.GetParametersByName(ctx, map[string]ssm.GetOptions{"key": {}}, zero); e != nil {
		t.Fatal(e)
	}
	if f.batches != 2 {
		t.Fatalf("warm zero-age batch hit cache: %d", f.batches)
	}
	if _, e := p.GetParametersByName(ctx, map[string]ssm.GetOptions{"cold": {}}, zero); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Get(ctx, "cold", ssm.GetOptions{Decrypt: aws.Bool(false)}); e != nil {
		t.Fatal(e)
	}
	if f.singles != 2 {
		t.Fatalf("zero-age batch saved a cache entry: %d", f.singles)
	}
}

func TestNonpositiveLifetimeBypassesWarmCache(t *testing.T) {
	for _, multiple := range []bool{false, true} {
		for _, age := range []time.Duration{0, -time.Second} {
			t.Run(fmt.Sprintf("%t/%s", multiple, age), func(t *testing.T) {
				c := parameters.NewCache(nil)
				calls := 0
				ctx := context.Background()
				get := func(o parameters.Options) {
					t.Helper()
					var e error
					if multiple {
						_, e = c.GetMultiple(ctx, "key", o, func(context.Context) (map[string]any, error) { calls++; return map[string]any{"n": calls}, nil })
					} else {
						_, e = c.Get(ctx, "key", o, func(context.Context) (any, error) { calls++; return calls, nil })
					}
					if e != nil {
						t.Fatal(e)
					}
				}
				positive := parameters.Options{MaxAge: parameters.Age(time.Minute)}
				get(positive)
				get(parameters.Options{MaxAge: parameters.Age(age)})
				get(parameters.Options{MaxAge: parameters.Age(age)})
				get(positive)
				if calls != 3 {
					t.Fatalf("nonpositive lifetime reused or evicted warm entry: calls=%d", calls)
				}
			})
		}
	}
	c := parameters.NewCache(nil)
	positive := parameters.Options{MaxAge: parameters.Age(time.Minute)}
	c.Store("key", "old", positive)
	for _, age := range []time.Duration{0, -time.Second} {
		o := parameters.Options{MaxAge: parameters.Age(age)}
		if _, ok := c.Lookup("key", o); ok {
			t.Fatal("nonpositive Lookup hit")
		}
		c.Store("key", "new", o)
	}
	if v, ok := c.Lookup("key", positive); !ok || v != "old" {
		t.Fatalf("nonpositive Store overwrote cache: %v/%t", v, ok)
	}
	t.Setenv("POWERTOOLS_PARAMETERS_MAX_AGE", "0")
	if _, ok := c.Lookup("key", parameters.Options{}); ok {
		t.Fatal("zero environment Lookup hit")
	}
}
