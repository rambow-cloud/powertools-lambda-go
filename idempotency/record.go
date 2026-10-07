package idempotency

import (
	"context"
	jsonv1 "encoding/json"
	"errors"
	"fmt"
	"time"
)

type Status string

const (
	InProgress Status = "INPROGRESS"
	Completed  Status = "COMPLETED"
	Expired    Status = "EXPIRED"
)

// Record uses the reference persistence field names. Expiration is in Unix
// seconds; InProgressExpiration is in Unix milliseconds. Zero means absent.
// Data holds JSON response bytes, not a JSON string containing those bytes.
type Record struct {
	Key                  string            `json:"id"`
	Status               Status            `json:"status"`
	Expiration           int64             `json:"expiration,omitzero"`
	InProgressExpiration int64             `json:"in_progress_expiration,omitzero"`
	Data                 jsonv1.RawMessage `json:"data,omitempty"`
	Validation           string            `json:"validation,omitempty"`
}

func (r Record) IsExpired(now time.Time) bool {
	return r.Expiration != 0 && now.After(time.Unix(r.Expiration, 0))
}

func (r Record) CurrentStatus(now time.Time) (Status, error) {
	if r.IsExpired(now) {
		return Expired, nil
	}
	switch r.Status {
	case InProgress, Completed, Expired:
		return r.Status, nil
	default:
		return "", &InvalidStatusError{r.Status}
	}
}

func (r Record) Clone() Record {
	r.Data = append(jsonv1.RawMessage(nil), r.Data...)
	return r
}

// Store owns atomic acquisition, response persistence, and cleanup. Put must
// reject a live existing record with AlreadyExistsError, optionally carrying
// its snapshot. Expired records and expired execution leases may be replaced.
// Get must provide a consistent snapshot. Implementations must be concurrency-safe.
// Update and Delete follow the reference's unconditional write semantics.
type Store interface {
	Put(context.Context, Record, time.Time) error
	Get(context.Context, string) (Record, error)
	Update(context.Context, Record) error
	Delete(context.Context, string) error
}

var ErrNotFound = errors.New("idempotency record not found")

type AlreadyExistsError struct{ Record *Record }

func (*AlreadyExistsError) Error() string { return "idempotency record already exists" }

type AlreadyInProgressError struct{ Key string }

func (e *AlreadyInProgressError) Error() string { return "execution already in progress: " + e.Key }

type InconsistentStateError struct{ Key string }

func (e *InconsistentStateError) Error() string {
	return "idempotency state changed during acquisition: " + e.Key
}

type ValidationError struct{ Record Record }

func (*ValidationError) Error() string {
	return "payload does not match stored record for this event key"
}

type KeyError struct{ Err error }

func (e *KeyError) Error() string { return "cannot create idempotency key: " + e.Err.Error() }
func (e *KeyError) Unwrap() error { return e.Err }

type ConfigurationError struct{ Message string }

func (e *ConfigurationError) Error() string { return e.Message }

type InvalidStatusError struct{ Status Status }

func (e *InvalidStatusError) Error() string {
	return fmt.Sprintf("invalid idempotency status %q", e.Status)
}

type PersistenceError struct {
	Operation string
	Err       error
}

func (e *PersistenceError) Error() string { return "idempotency " + e.Operation + ": " + e.Err.Error() }
func (e *PersistenceError) Unwrap() error { return e.Err }
