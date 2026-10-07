package idempotency

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"hash"
	"log"
	"math"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/commons"
	"github.com/rambow-cloud/powertools-lambda-go/internal/invocation"
	"github.com/rambow-cloud/powertools-lambda-go/jmespath"
)

type Query interface{ Search(any) (any, error) }

// Options are snapshotted at construction. Query, serializer, clock, and
// diagnostic callbacks must be concurrency-safe. A nil duration uses one hour.
type Options struct {
	KeyPrefix                 string
	EventKeyJMESPath          string
	PayloadValidationJMESPath string
	KeyQuery                  Query
	ValidationQuery           Query
	ThrowOnNoKey              bool
	ExpiresAfter              *time.Duration
	HashFunction              string
	SerializeKey              func(any) ([]byte, error)
	UseLocalCache             bool
	MaxLocalCacheSize         int
	Disabled                  bool
	Now                       func() time.Time
	Diagnostic                func(string)
}

// Manager owns immutable operation configuration and an optional response cache.
// Multiple managers may share a Store while retaining independent key prefixes,
// queries, deadlines, and caches. It never stores invocation context globally.
type Manager struct {
	store                     Store
	prefix                    string
	keyQuery, validationQuery Query
	throwOnNoKey, disabled    bool
	ttl                       time.Duration
	newHash                   func() hash.Hash
	serialize                 func(any) ([]byte, error)
	now                       func() time.Time
	diagnostic                func(string)
	cache                     *commons.LRUCache[string, Record]
}

func New(store Store, options Options) (*Manager, error) {
	if store == nil {
		return nil, &ConfigurationError{"persistence store is required"}
	}
	if options.MaxLocalCacheSize < 0 {
		return nil, &ConfigurationError{"cache size must not be negative"}
	}
	m := &Manager{store: store, prefix: commons.TrimSpace(options.KeyPrefix), keyQuery: options.KeyQuery, validationQuery: options.ValidationQuery, throwOnNoKey: options.ThrowOnNoKey, disabled: options.Disabled, ttl: time.Hour, serialize: options.SerializeKey, now: options.Now, diagnostic: options.Diagnostic}
	if m.prefix == "" {
		m.prefix, _ = commons.StringEnv("AWS_LAMBDA_FUNCTION_NAME", "")
	}
	if options.ExpiresAfter != nil {
		m.ttl = *options.ExpiresAfter
	}
	if m.ttl <= 0 {
		return nil, &ConfigurationError{"ExpiresAfter must be positive"}
	}
	if m.now == nil {
		m.now = time.Now
	}
	if m.diagnostic == nil {
		m.diagnostic = func(message string) { log.Print(message) }
	}
	var err error
	if m.keyQuery == nil && options.EventKeyJMESPath != "" {
		m.keyQuery, err = jmespath.Compile(options.EventKeyJMESPath, jmespath.WithPowertoolsFunctions())
		if err != nil {
			return nil, err
		}
	}
	if m.validationQuery == nil && options.PayloadValidationJMESPath != "" {
		m.validationQuery, err = jmespath.Compile(options.PayloadValidationJMESPath, jmespath.WithPowertoolsFunctions())
		if err != nil {
			return nil, err
		}
	}
	switch options.HashFunction {
	case "", "md5":
		m.newHash = md5.New
	case "sha1":
		m.newHash = sha1.New
	case "sha256":
		m.newHash = sha256.New
	case "sha384":
		m.newHash = sha512.New384
	case "sha512":
		m.newHash = sha512.New
	default:
		return nil, &ConfigurationError{"unsupported hash function: " + options.HashFunction}
	}
	disabled, err := commons.BoolEnv("POWERTOOLS_IDEMPOTENCY_DISABLED", true, false)
	if err != nil {
		return nil, err
	}
	m.disabled = m.disabled || disabled
	if options.UseLocalCache {
		capacity := options.MaxLocalCacheSize
		if capacity == 0 {
			capacity = 1000
		}
		m.cache = commons.NewLRUCache[string, Record](capacity)
	}
	return m, nil
}

func (m *Manager) ClearCache() {
	if m.cache != nil {
		m.cache.Clear()
	}
}

func (m *Manager) digest(value any) (string, error) {
	serialize := m.serialize
	if serialize == nil {
		serialize = CanonicalJSON
	}
	encoded, err := serialize(value)
	if err != nil {
		return "", &KeyError{err}
	}
	h := m.newHash()
	_, _ = h.Write(encoded)
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// Key returns the persistence key, validation hash, and whether an absent query
// selection should bypass idempotency. False/zero/empty selections are hashed
// unless ThrowOnNoKey is enabled, matching the reference's two separate checks.
func (m *Manager) Key(payload any) (key, validation string, skip bool, err error) {
	selection := payload
	if m.keyQuery != nil {
		selection, err = m.keyQuery.Search(payload)
		if err != nil {
			return "", "", false, &KeyError{err}
		}
		if selection == nil && !m.throwOnNoKey {
			return "", "", true, nil
		}
	}
	encoded, err := json.Marshal(selection)
	if err != nil {
		return "", "", false, &KeyError{err}
	}
	var normalized any
	if err = json.Unmarshal(encoded, &normalized); err != nil {
		return "", "", false, &KeyError{err}
	}
	if missingKey(normalized) {
		if m.throwOnNoKey {
			return "", "", false, &KeyError{fmt.Errorf("no data found to create a hashed idempotency key")}
		}
		m.diagnostic("No value found for idempotency_key")
	}
	// Reuse the snapshot used by missing-key validation. Invoking a custom
	// marshaler twice could hash a different value or duplicate its side effects.
	if m.serialize == nil {
		selection = jsonv1.RawMessage(encoded)
	}
	hash, err := m.digest(selection)
	if err != nil {
		return "", "", false, err
	}
	if m.validationQuery != nil {
		value, queryErr := m.validationQuery.Search(payload)
		if queryErr != nil {
			return "", "", false, &KeyError{queryErr}
		}
		validation, err = m.digest(value)
		if err != nil {
			return "", "", false, err
		}
	}
	return m.prefix + "#" + hash, validation, false, nil
}

func missingKey(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for _, item := range v {
			if truthy(item) {
				return false
			}
		}
		return true
	case []any:
		for _, item := range v {
			if truthy(item) {
				return false
			}
		}
		return true
	default:
		return !truthy(v)
	}
}
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	default:
		return true
	}
}

func (m *Manager) expiry() int64 {
	return int64(math.Floor(float64(m.now().UnixMilli())/1000 + m.ttl.Seconds() + 0.5))
}

func (m *Manager) existing(ctx context.Context, proposed Record) (Record, bool, error) {
	var existing *Record
	if m.cache != nil {
		if cached, ok := m.cache.Get(proposed.Key); ok {
			if cached.IsExpired(m.now()) {
				m.cache.Remove(proposed.Key)
			} else {
				copy := cached.Clone()
				existing = &copy
			}
		}
	}
	if existing == nil {
		err := m.store.Put(ctx, proposed.Clone(), m.now())
		if err == nil {
			return Record{}, false, nil
		}
		var conflict *AlreadyExistsError
		if !errors.As(err, &conflict) {
			return Record{}, false, &PersistenceError{"acquire", err}
		}
		existing = conflict.Record
		if existing == nil {
			record, err := m.store.Get(ctx, proposed.Key)
			if errors.Is(err, ErrNotFound) {
				return Record{}, false, &InconsistentStateError{proposed.Key}
			}
			if err != nil {
				return Record{}, false, &PersistenceError{"get", err}
			}
			existing = &record
		}
	}
	if m.validationQuery != nil && existing.Validation != proposed.Validation {
		return Record{}, false, &ValidationError{existing.Clone()}
	}
	status, err := existing.CurrentStatus(m.now())
	if err != nil {
		return Record{}, false, err
	}
	switch status {
	case Expired:
		return Record{}, false, &InconsistentStateError{proposed.Key}
	case InProgress:
		if existing.InProgressExpiration != 0 && existing.InProgressExpiration < m.now().UnixMilli() {
			return Record{}, false, &InconsistentStateError{proposed.Key}
		}
		return Record{}, false, &AlreadyInProgressError{proposed.Key}
	}
	if m.cache != nil {
		m.cache.Add(proposed.Key, existing.Clone())
	}
	return existing.Clone(), true, nil
}

// Execute makes one operation idempotent. The payload may be a projection of
// the handler arguments. The callback must honor context deadlines. A replay
// hook runs only for stored responses, and receives a private record snapshot.
func Execute[R any](ctx context.Context, m *Manager, payload any, handler func(context.Context) (R, error), replayHook ...func(context.Context, R, Record) (R, error)) (result R, err error) {
	if m == nil || handler == nil {
		return result, &ConfigurationError{"manager and handler are required"}
	}
	ctx = invocation.Ensure(ctx)
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if m.disabled {
		return handler(ctx)
	}
	key, validation, skip, err := m.Key(payload)
	if err != nil {
		return result, err
	}
	if skip {
		return handler(ctx)
	}
	proposed := Record{Key: key, Validation: validation, Status: InProgress, Expiration: m.expiry()}
	if deadline, ok := ctx.Deadline(); ok {
		proposed.InProgressExpiration = deadline.UnixMilli()
	} else {
		m.diagnostic("Could not determine remaining execution time; provide a context deadline")
	}
	for attempt := 0; ; attempt++ {
		record, found, acquireErr := m.existing(ctx, proposed)
		var inconsistent *InconsistentStateError
		if errors.As(acquireErr, &inconsistent) && attempt < 2 {
			continue
		}
		if acquireErr != nil {
			return result, acquireErr
		}
		if !found {
			break
		}
		if err = json.Unmarshal(record.Data, &result); err != nil {
			return result, &PersistenceError{"decode response", err}
		}
		if len(replayHook) > 0 && replayHook[0] != nil {
			return replayHook[0](ctx, result, record)
		}
		return result, nil
	}
	// Cleanup uses the caller's deadline. If it has expired, the persisted lease
	// enables later recovery; a detached background write could race that recovery.
	result, err = func() (R, error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if cleanup := m.store.Delete(ctx, key); cleanup != nil {
					m.diagnostic("Failed to delete idempotency record after panic: " + cleanup.Error())
				}
				panic(recovered)
			}
		}()
		return handler(ctx)
	}()
	if err != nil {
		if cleanup := m.store.Delete(ctx, key); cleanup != nil {
			return result, errors.Join(err, &PersistenceError{"delete", cleanup})
		}
		return result, err
	}
	encoded, encodeErr := json.Marshal(result)
	if encodeErr != nil {
		return result, &PersistenceError{"encode response", encodeErr}
	}
	proposed.Status = Completed
	proposed.Data = encoded
	proposed.Expiration = m.expiry()
	proposed.InProgressExpiration = 0
	if err = m.store.Update(ctx, proposed.Clone()); err != nil {
		return result, &PersistenceError{"complete", err}
	}
	if m.cache != nil {
		m.cache.Add(key, proposed.Clone())
	}
	return result, nil
}

func WrapHandler[T, R any](m *Manager, handler func(context.Context, T) (R, error), replayHook ...func(context.Context, R, Record) (R, error)) func(context.Context, T) (R, error) {
	return func(ctx context.Context, event T) (R, error) {
		if handler == nil {
			var zero R
			return zero, &ConfigurationError{"handler is required"}
		}
		return Execute(ctx, m, event, func(ctx context.Context) (R, error) { return handler(ctx, event) }, replayHook...)
	}
}
