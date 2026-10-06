package parameters

import (
	"context"
	"errors"
	"fmt"
	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"sync"
	"time"
)

// Options controls one retrieval. A nil MaxAge reads the environment on each call.
// Nonpositive MaxAge bypasses lookup and storage; ForceFetch bypasses lookup.
type Options struct {
	// RequestKey isolates effective provider request options. Providers set it automatically.
	RequestKey            string
	MaxAge                *time.Duration
	ForceFetch            bool
	Transform             Transform
	ThrowOnMissing        bool
	ThrowOnTransformError bool
}

// Age returns a duration pointer for an explicit cache lifetime, including zero.
func Age(d time.Duration) *time.Duration { return &d }

func (o Options) Lifetime() time.Duration {
	age, err := o.lifetime()
	if err != nil {
		return 5 * time.Second
	}
	return age
}

func (o Options) lifetime() (time.Duration, error) {
	if o.MaxAge != nil {
		return *o.MaxAge, nil
	}
	seconds, err := commons.NumberEnv("POWERTOOLS_PARAMETERS_MAX_AGE", 5)
	if err != nil {
		return 0, err
	}
	if seconds <= 0 {
		return 0, nil
	}
	if seconds >= float64(time.Duration(1<<63-1))/float64(time.Second) {
		return time.Duration(1<<63 - 1), nil
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

// GetParameterError preserves the underlying SDK or context error for errors.Is/As.
type GetParameterError struct {
	Name string
	Err  error
}

func (e *GetParameterError) Error() string {
	return fmt.Sprintf("unable to get parameter %q: %v", e.Name, e.Err)
}
func (e *GetParameterError) Unwrap() error { return e.Err }

type ParameterNotFoundError struct{ Name string }

func (e *ParameterNotFoundError) Error() string { return fmt.Sprintf("parameter %q not found", e.Name) }

type SetParameterError struct {
	Name string
	Err  error
}

func (e *SetParameterError) Error() string {
	return fmt.Sprintf("unable to set parameter %q: %v", e.Name, e.Err)
}
func (e *SetParameterError) Unwrap() error { return e.Err }

// GetError normalizes retrieval errors without losing typed library errors.
func GetError(name string, err error) error {
	if err == nil {
		return nil
	}
	var get *GetParameterError
	var missing *ParameterNotFoundError
	var transform *TransformParameterError
	if errors.As(err, &get) || errors.As(err, &missing) || errors.As(err, &transform) {
		return err
	}
	return &GetParameterError{Name: name, Err: err}
}

type cacheKey struct {
	request   string
	name      string
	transform Transform
	multiple  bool
}
type entry struct {
	value   any
	expires time.Time
}

// Cache is safe for concurrent retrievals. Construct one per provider, not per invocation.
// RequestKey isolates value-affecting request options within each operation.
// Concurrent misses may fetch independently, matching the reference provider.
type Cache struct {
	mu         sync.Mutex
	entries    map[cacheKey]entry
	clock      func() time.Time
	generation uint64
}

// NewCache accepts an optional clock for deterministic expiry tests.
func NewCache(clock func() time.Time) *Cache {
	if clock == nil {
		clock = time.Now
	}
	return &Cache{entries: make(map[cacheKey]entry), clock: clock}
}

// ClearCache also prevents in-flight Get/GetMultiple callbacks from repopulating entries.
func (c *Cache) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.entries)
	c.generation++
}

func (c *Cache) lookup(key cacheKey, force bool) (any, bool, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.entries[key]
	if ok && value.expires.Before(c.clock()) {
		delete(c.entries, key)
		ok = false
	}
	if force || !ok {
		return nil, false, c.generation
	}
	return commons.CloneValue(value.value), true, c.generation
}

func (c *Cache) save(key cacheKey, value any, age time.Duration, generation uint64) {
	if age <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if generation == c.generation {
		c.entries[key] = entry{commons.CloneValue(value), c.clock().Add(age)}
	}
}

// Lookup and Store share single-value entries with batched provider operations.
func (c *Cache) Lookup(name string, options Options) (any, bool) {
	age, err := options.lifetime()
	if err != nil || age <= 0 {
		return nil, false
	}
	value, ok, _ := c.lookup(cacheKey{request: options.RequestKey, name: name, transform: options.Transform}, options.ForceFetch)
	return value, ok
}

// Store caches an already transformed value returned by a batched operation.
func (c *Cache) Store(name string, value any, options Options) {
	age, err := options.lifetime()
	if err != nil || age <= 0 {
		return
	}
	key := cacheKey{request: options.RequestKey, name: name, transform: options.Transform}
	_, _, generation := c.lookup(key, true)
	c.save(key, value, age, generation)
}

// Get fetches and transforms a value on a cache miss. A nil value means missing.
func (c *Cache) Get(ctx context.Context, name string, options Options, fetch func(context.Context) (any, error)) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, GetError(name, err)
	}
	key := cacheKey{request: options.RequestKey, name: name, transform: options.Transform}
	age, err := options.lifetime()
	if err != nil {
		return nil, GetError(name, err)
	}
	if value, ok, generation := c.lookup(key, options.ForceFetch || age <= 0); ok {
		return value, nil
	} else {
		value, err := fetch(ctx)
		if err != nil {
			return nil, GetError(name, err)
		}
		if err = ctx.Err(); err != nil {
			return nil, GetError(name, err)
		}
		if value == nil {
			if options.ThrowOnMissing {
				return nil, &ParameterNotFoundError{Name: name}
			}
			return nil, nil
		}
		value, err = TransformValue(name, value, options.Transform)
		if err != nil {
			return nil, err
		}
		c.save(key, value, age, generation)
		return value, nil
	}
}

func (c *Cache) GetMultiple(ctx context.Context, path string, options Options, fetch func(context.Context) (map[string]any, error)) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, GetError(path, err)
	}
	key := cacheKey{request: options.RequestKey, name: path, transform: options.Transform, multiple: true}
	age, err := options.lifetime()
	if err != nil {
		return nil, GetError(path, err)
	}
	value, ok, generation := c.lookup(key, options.ForceFetch || age <= 0)
	if ok {
		return value.(map[string]any), nil
	}
	values, err := fetch(ctx)
	if err != nil {
		return nil, GetError(path, err)
	}
	if err = ctx.Err(); err != nil {
		return nil, GetError(path, err)
	}
	result := make(map[string]any, len(values))
	for name, raw := range values {
		transformed, err := TransformValue(name, raw, options.Transform)
		if err != nil && options.ThrowOnTransformError {
			return nil, err
		}
		result[name] = transformed
	}
	if len(result) > 0 {
		c.save(key, result, age, generation)
	}
	return result, nil
}
