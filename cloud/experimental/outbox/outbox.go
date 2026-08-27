/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package outbox implements the transactional outbox pattern: business writes
// and message publishes become atomic by writing the message to an outbox
// table inside the same database transaction, then a [Relay] drains the table
// to a real broker in the background.
//
// Delivery semantics are at-least-once: if the process dies between a
// successful publish and [Store.MarkSent], the message is delivered again.
// Consumers must be idempotent (or key messages and de-duplicate).
//
// This package is broker-neutral: the Relay publishes through a
// [messaging.Binder], so any broker starter (kafka, nats, ...) works as the
// delivery side. The write side — inserting into the outbox table inside the
// business transaction — lives with each storage backend starter (e.g.
// starter-outbox-gorm), because it needs the transaction handle.
package outbox

import (
	"context"
	"time"
)

// Record is the broker-neutral projection of one outbox row.
type Record struct {
	// ID is the monotonic sequence anchor; the Relay delivers in ID order
	// within a batch, so the backend should allocate IDs monotonically.
	ID int64

	// Destination is the binder destination (topic / subject / queue name).
	Destination string

	// Key is the optional partitioning / ordering key passed through to the
	// binder; brokers with keyed streams keep same-key messages ordered.
	Key string

	// Payload is the opaque message body.
	Payload []byte

	// Headers carries string metadata, passed through to the binder and
	// doubled as the trace-context carrier, as in [messaging.Message].
	Headers map[string]string

	// Attempts is how many delivery attempts have failed so far.
	Attempts int

	// LastError is the error message of the most recent failed attempt.
	LastError string

	// CreatedAt is when the record was written by the business transaction.
	CreatedAt time.Time
}

// Record lifecycle: pending → sent | dead.
const (
	// StatusPending means the record is waiting for (re)delivery.
	StatusPending = "pending"

	// StatusSent means the record was published successfully; it is kept for
	// audit until archived by the operator.
	StatusSent = "sent"

	// StatusDead means delivery attempts were exhausted and the record was
	// dead-lettered (or directly terminal when DLQ is disabled).
	StatusDead = "dead"
)

// Store persists outbox records. Implementations must tolerate multiple relay
// instances running concurrently: two simultaneous [Store.Fetch] calls must not
// return the same pending record (the gorm backend uses FOR UPDATE SKIP
// LOCKED; single-writer backends such as SQLite satisfy this trivially).
type Store interface {
	// Fetch returns at most limit pending records whose retry is due
	// (next_retry_at <= now), ordered by ID ascending.
	Fetch(ctx context.Context, now time.Time, limit int) ([]Record, error)

	// MarkSent records a successful publish.
	MarkSent(ctx context.Context, id int64, sentAt time.Time) error

	// MarkFailed records one failed attempt; nextRetryAt is computed by the
	// Relay's backoff policy and stored verbatim.
	MarkFailed(ctx context.Context, id int64, err error, nextRetryAt time.Time) error

	// MarkDead terminally retires the record; the Relay calls it only after
	// the DLQ copy was published (or when DLQ is disabled).
	MarkDead(ctx context.Context, id int64, err error) error
}

// Config is the full set of Relay knobs. The zero value is usable: 1s poll,
// batches of 100, 8 attempts, backoff 1s doubling capped at 1m, DLQ suffix
// ".dlq" (empty disables DLQ: exhausted records go straight to dead).
type Config struct {
	// PollInterval is the wait between polls when the store is drained.
	// Values below 100ms are clamped to 100ms.
	PollInterval time.Duration

	// BatchSize is the maximum number of records fetched per poll.
	BatchSize int

	// MaxAttempts is the total delivery attempts before a record is
	// dead-lettered. Zero means 8; negative is treated as 1.
	MaxAttempts int

	// BackoffBase is the wait after the first failure; each further failure
	// doubles it up to BackoffMax.
	BackoffBase time.Duration

	// BackoffMax caps a single backoff wait.
	BackoffMax time.Duration

	// DLQSuffix is appended to the destination to derive the dead-letter
	// destination ("orders" → "orders.dlq"). Empty disables the DLQ: exhausted
	// records are marked dead without a copy.
	DLQSuffix string
}

// withDefaults returns cfg normalized as documented.
func (c Config) withDefaults() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	} else if c.PollInterval < 100*time.Millisecond {
		c.PollInterval = 100 * time.Millisecond
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if c.MaxAttempts == 0 {
		c.MaxAttempts = 8
	} else if c.MaxAttempts < 0 {
		c.MaxAttempts = 1
	}
	if c.BackoffBase <= 0 {
		c.BackoffBase = time.Second
	}
	if c.BackoffMax <= 0 {
		c.BackoffMax = time.Minute
	}
	return c
}

// backoff returns the retry wait after attempts failed failures, doubling
// BackoffBase and capping at BackoffMax.
func (c Config) backoff(attempts int) time.Duration {
	d := c.BackoffBase
	for i := 1; i < attempts && d < c.BackoffMax; i++ {
		d *= 2
	}
	if d > c.BackoffMax {
		d = c.BackoffMax
	}
	return d
}

// Observer receives relay lifecycle events. It is the observability seam:
// this package imports no tracing library; a starter or example provides the
// adapter. Implementations must not block; events are emitted synchronously
// from the relay loop.
type Observer interface {
	// OnPublished fires after a record was successfully delivered.
	OnPublished(rec *Record)

	// OnRetry fires after a failed attempt that did not exhaust MaxAttempts.
	OnRetry(rec *Record, err error, nextRetry time.Time)

	// OnDead fires when a record was dead-lettered (or terminal without DLQ).
	OnDead(rec *Record, err error)
}

// ObserverFunc adapts a function to the corresponding Observer event; nil
// receivers are dropped, so partial observers can be composed by hand:
//
//	outbox.ObserverFunc(nil, onRetry, onDead)
type ObserverFunc struct {
	// OnPublishedFunc, when non-nil, handles OnPublished.
	OnPublishedFunc func(rec *Record)

	// OnRetryFunc, when non-nil, handles OnRetry.
	OnRetryFunc func(rec *Record, err error, nextRetry time.Time)

	// OnDeadFunc, when non-nil, handles OnDead.
	OnDeadFunc func(rec *Record, err error)
}

// OnPublished implements [Observer].
func (o ObserverFunc) OnPublished(rec *Record) {
	if o.OnPublishedFunc != nil {
		o.OnPublishedFunc(rec)
	}
}

// OnRetry implements [Observer].
func (o ObserverFunc) OnRetry(rec *Record, err error, nextRetry time.Time) {
	if o.OnRetryFunc != nil {
		o.OnRetryFunc(rec, err, nextRetry)
	}
}

// OnDead implements [Observer].
func (o ObserverFunc) OnDead(rec *Record, err error) {
	if o.OnDeadFunc != nil {
		o.OnDeadFunc(rec, err)
	}
}
