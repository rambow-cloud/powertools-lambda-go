// Package cache provides Redis/Valkey persistence for idempotent operations.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rambow-cloud/powertools-lambda-go/idempotency"
	"github.com/redis/go-redis/v9"
)

// Client is implemented by go-redis standalone, Sentinel and cluster clients.
// The caller configures credentials, TLS, deadlines and retries, and owns Close.
type Client interface {
	Get(context.Context, string) *redis.StringCmd
	SetArgs(context.Context, string, any, redis.SetArgs) *redis.StatusCmd
	Del(context.Context, ...string) *redis.IntCmd
	Eval(context.Context, string, []string, ...any) *redis.Cmd
}

type Options struct {
	StatusAttribute, ExpiryAttribute, InProgressExpiryAttribute string
	DataAttribute, ValidationAttribute                          string
	Now                                                         func() time.Time
	// OmitValidationOnSuccess reproduces the pinned TypeScript cache writer's
	// omission. Leave false to retain validation when replaying completed records.
	OmitValidationOnSuccess bool
}

type Store struct {
	client  Client
	options Options
}

func New(client Client, options Options) (*Store, error) {
	if client == nil {
		return nil, &idempotency.ConfigurationError{Message: "cache client is required"}
	}
	fields := []struct {
		target   *string
		fallback string
	}{
		{&options.StatusAttribute, "status"}, {&options.ExpiryAttribute, "expiration"},
		{&options.InProgressExpiryAttribute, "in_progress_expiration"},
		{&options.DataAttribute, "data"}, {&options.ValidationAttribute, "validation"},
	}
	seen := map[string]bool{}
	for _, field := range fields {
		if *field.target == "" {
			*field.target = field.fallback
		}
		if seen[*field.target] {
			return nil, &idempotency.ConfigurationError{Message: "cache attribute names must be distinct"}
		}
		seen[*field.target] = true
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Store{client: client, options: options}, nil
}

func ttl(record idempotency.Record, now time.Time) (time.Duration, error) {
	seconds := record.Expiration - now.Unix()
	if seconds <= 0 || seconds > int64((time.Duration(1<<63-1))/time.Second) {
		return 0, &idempotency.ConfigurationError{Message: "cache record requires a positive representable TTL"}
	}
	return time.Duration(seconds) * time.Second, nil
}

func (s *Store) encode(record idempotency.Record, completed bool) ([]byte, error) {
	o := s.options
	item := map[string]any{o.StatusAttribute: record.Status, o.ExpiryAttribute: record.Expiration}
	if completed {
		if len(record.Data) > 0 {
			item[o.DataAttribute] = record.Data
		}
	} else if record.InProgressExpiration != 0 {
		item[o.InProgressExpiryAttribute] = record.InProgressExpiration
	}
	if record.Validation != "" && (!completed || !o.OmitValidationOnSuccess) {
		item[o.ValidationAttribute] = record.Validation
	}
	return json.Marshal(item)
}

func (s *Store) decode(key, value string) (idempotency.Record, error) {
	r := idempotency.Record{Key: key}
	var item map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &item); err != nil {
		return r, &ConsistencyError{err}
	}
	if item == nil {
		return r, &ConsistencyError{fmt.Errorf("cache value must be an object")}
	}
	o := s.options
	for _, field := range []struct {
		name     string
		target   any
		required bool
	}{
		{o.StatusAttribute, &r.Status, true}, {o.ExpiryAttribute, &r.Expiration, false},
		{o.InProgressExpiryAttribute, &r.InProgressExpiration, false}, {o.ValidationAttribute, &r.Validation, false},
	} {
		encoded, exists := item[field.name]
		if !exists {
			if field.required {
				return r, &ConsistencyError{fmt.Errorf("missing attribute %s", field.name)}
			}
			continue
		}
		if err := json.Unmarshal(encoded, field.target); err != nil {
			return r, &ConsistencyError{fmt.Errorf("invalid attribute %s: %w", field.name, err)}
		}
	}
	r.Data = append(json.RawMessage(nil), item[o.DataAttribute]...)
	return r, nil
}

func (s *Store) Get(ctx context.Context, key string) (idempotency.Record, error) {
	value, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return idempotency.Record{}, idempotency.ErrNotFound
	}
	if err != nil {
		return idempotency.Record{}, err
	}
	return s.decode(key, value)
}

// Recover only the exact value inspected by this caller. The single-key script
// works across cluster slots independently of the reference key:lock lease.
const recoverScript = `if redis.call('GET', KEYS[1]) == ARGV[1] then
  redis.call('SET', KEYS[1], ARGV[2], 'EX', ARGV[3])
  return 1
end
return 0`

func (s *Store) Put(ctx context.Context, record idempotency.Record, now time.Time) error {
	if record.Status != idempotency.InProgress {
		return &idempotency.InvalidStatusError{Status: record.Status}
	}
	duration, err := ttl(record, now)
	if err != nil {
		return err
	}
	encoded, err := s.encode(record, false)
	if err != nil {
		return err
	}
	_, err = s.client.SetArgs(ctx, record.Key, string(encoded), redis.SetArgs{Mode: "NX", TTL: duration}).Result()
	if err == nil {
		return nil
	}
	if !errors.Is(err, redis.Nil) {
		return err
	}
	raw, err := s.client.Get(ctx, record.Key).Result()
	if errors.Is(err, redis.Nil) {
		return &idempotency.AlreadyExistsError{}
	}
	if err != nil {
		return err
	}
	existing, decodeErr := s.decode(record.Key, raw)
	if decodeErr == nil {
		status, statusErr := existing.CurrentStatus(now)
		if statusErr != nil {
			return statusErr
		}
		// An absent execution deadline is not evidence of an abandoned operation.
		if status == idempotency.Completed || (status == idempotency.InProgress && (existing.InProgressExpiration == 0 || existing.InProgressExpiration > now.UnixMilli())) {
			return &idempotency.AlreadyExistsError{Record: &existing}
		}
	}
	// The reference retains a ten-second orphan lock, including after recovery.
	_, err = s.client.SetArgs(ctx, record.Key+":lock", "true", redis.SetArgs{Mode: "NX", TTL: 10 * time.Second}).Result()
	if errors.Is(err, redis.Nil) {
		return &idempotency.AlreadyExistsError{}
	}
	if err != nil {
		return err
	}
	replaced, err := s.client.Eval(ctx, recoverScript, []string{record.Key}, raw, string(encoded), int64(duration/time.Second)).Int64()
	if err != nil {
		return err
	}
	if replaced != 1 {
		return &idempotency.AlreadyExistsError{}
	}
	return nil
}

func (s *Store) Update(ctx context.Context, record idempotency.Record) error {
	duration, err := ttl(record, s.options.Now())
	if err != nil {
		return err
	}
	encoded, err := s.encode(record, true)
	if err != nil {
		return err
	}
	return s.client.SetArgs(ctx, record.Key, string(encoded), redis.SetArgs{TTL: duration}).Err()
}

func (s *Store) Delete(ctx context.Context, key string) error { return s.client.Del(ctx, key).Err() }

type ConsistencyError struct{ Err error }

func (e *ConsistencyError) Error() string {
	return "invalid cache idempotency record: " + e.Err.Error()
}
func (e *ConsistencyError) Unwrap() error { return e.Err }

var _ idempotency.Store = (*Store)(nil)
var _ Client = (*redis.Client)(nil)
var _ Client = (*redis.ClusterClient)(nil)
