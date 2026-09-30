// Package invocation shares invocation identity across utility wrappers.
package invocation

import (
	"context"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"sync/atomic"
)

type key struct{}

// Info belongs to one handler invocation and is immutable after construction.
type Info struct{ ColdStart bool }

var invoked atomic.Bool

// Ensure reuses invocation information when utility wrappers are composed.
func Ensure(ctx context.Context) context.Context {
	if _, ok := ctx.Value(key{}).(Info); ok {
		return ctx
	}
	return context.WithValue(ctx, key{}, Info{ColdStart: !invoked.Swap(true)})
}

func FromContext(ctx context.Context) (Info, bool) {
	v, ok := ctx.Value(key{}).(Info)
	return v, ok
}

// TraceHeader prefers the request context used by aws-lambda-go over process state.
func TraceHeader(ctx context.Context) string {
	return commons.TraceHeader(ctx)
}

func TraceID(ctx context.Context) string {
	return commons.XRayTraceID(ctx)
}
