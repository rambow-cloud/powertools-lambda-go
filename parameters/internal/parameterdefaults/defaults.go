// Package parameterdefaults manages lazily initialized default providers.
package parameterdefaults

import (
	"context"
	"sync"
)

var registry struct {
	sync.Mutex
	clear []func()
}

// Lazy initializes one provider, retries failed initialization, and supports cancellation while waiting.
type Lazy[T interface{ ClearCache() }] struct {
	gate  chan struct{}
	value T
	ready bool
}

func New[T interface{ ClearCache() }]() *Lazy[T] {
	lazy := &Lazy[T]{gate: make(chan struct{}, 1)}
	registry.Lock()
	registry.clear = append(registry.clear, func() {
		lazy.gate <- struct{}{}
		defer func() { <-lazy.gate }()
		if lazy.ready {
			lazy.value.ClearCache()
		}
	})
	registry.Unlock()
	return lazy
}

func (l *Lazy[T]) Get(ctx context.Context, create func(context.Context) (T, error)) (T, error) {
	var zero T
	select {
	case l.gate <- struct{}{}:
	case <-ctx.Done():
		return zero, ctx.Err()
	}
	defer func() { <-l.gate }()
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if !l.ready {
		value, err := create(ctx)
		if err != nil {
			return zero, err
		}
		l.value, l.ready = value, true
	}
	return l.value, nil
}

func ClearCaches() {
	registry.Lock()
	callbacks := append([]func(){}, registry.clear...)
	registry.Unlock()
	for _, clear := range callbacks {
		clear()
	}
}
