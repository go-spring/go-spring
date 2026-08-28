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

package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-spring.org/cloud/experimental/messaging"
)

// memBinder is an in-memory [messaging.Binder]: publishers either capture
// messages or fail, by destination.
type memBinder struct {
	mu   sync.Mutex
	sent map[string][]*messaging.Message // destination → delivered messages

	// failDests makes publishing to these destinations fail until succeeded
	// times (0 = always fail).
	failDests map[string]int
	attempts  map[string]int
}

func newMemBinder(failDests map[string]int) *memBinder {
	return &memBinder{
		sent:      make(map[string][]*messaging.Message),
		failDests: failDests,
		attempts:  make(map[string]int),
	}
}

func (b *memBinder) NewPublisher(ctx context.Context, destination string) (messaging.Publisher, error) {
	return &memPublisher{b: b, dest: destination}, nil
}

func (b *memBinder) NewSubscriber(ctx context.Context, source, group string) (messaging.Subscriber, error) {
	return nil, errors.New("outbox test: subscriber not supported")
}

func (b *memBinder) delivered(dest string) []*messaging.Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]*messaging.Message(nil), b.sent[dest]...)
}

type memPublisher struct {
	b    *memBinder
	dest string
}

func (p *memPublisher) Publish(ctx context.Context, msg *messaging.Message) error {
	p.b.mu.Lock()
	defer p.b.mu.Unlock()
	p.b.attempts[p.dest]++
	if n, ok := p.b.failDests[p.dest]; ok {
		if n == 0 || p.b.attempts[p.dest] <= n {
			return errors.New("boom: " + p.dest)
		}
	}
	p.b.sent[p.dest] = append(p.b.sent[p.dest], msg)
	return nil
}

func (p *memPublisher) Close() error { return nil }

// runOnce drives one relay batch synchronously (bypassing Run's ticker).
func runOnce(t *testing.T, r *Relay) {
	t.Helper()
	require.NoError(t, r.runBatch(context.Background()))
}

func TestRelay_DeliversPending(t *testing.T) {
	store := &MemoryStore{}
	binder := newMemBinder(nil)
	store.Add(Record{Destination: "orders", Payload: []byte("m1")}, time.Time{})
	store.Add(Record{Destination: "orders", Payload: []byte("m2")}, time.Time{})

	r := NewRelay(store, binder, Config{PollInterval: time.Hour})
	runOnce(t, r)

	msgs := binder.delivered("orders")
	require.Len(t, msgs, 2)
	assert.Equal(t, "m1", string(msgs[0].Payload))
	assert.Equal(t, "m2", string(msgs[1].Payload)) // in-batch ID order
	assert.Len(t, store.Snapshot(StatusSent), 2)
	assert.Empty(t, store.Snapshot(StatusPending))
}

func TestRelay_FetchHonorsNextRetry(t *testing.T) {
	store := &MemoryStore{}
	binder := newMemBinder(nil)
	future := time.Now().Add(time.Hour)
	store.Add(Record{Destination: "orders", Payload: []byte("later")}, future)

	r := NewRelay(store, binder, Config{PollInterval: time.Hour})
	runOnce(t, r)

	assert.Empty(t, binder.delivered("orders"))
	assert.Len(t, store.Snapshot(StatusPending), 1)
}

func TestRelay_FailureRetriesWithBackoff(t *testing.T) {
	store := &MemoryStore{}
	// "orders" fails the first 2 attempts, succeeds on the 3rd.
	binder := newMemBinder(map[string]int{"orders": 2})
	store.Add(Record{Destination: "orders", Payload: []byte("m1")}, time.Time{})

	var retries []time.Time
	obs := ObserverFunc{OnRetryFunc: func(rec *Record, err error, nextRetry time.Time) {
		retries = append(retries, nextRetry)
	}}
	r := NewRelay(store, binder, Config{PollInterval: time.Hour, BackoffBase: time.Minute}, obs)

	runOnce(t, r) // attempt 1 fails
	assert.Empty(t, binder.delivered("orders"))
	pending := store.Snapshot(StatusPending)
	require.Len(t, pending, 1)
	assert.Equal(t, 1, pending[0].Attempts)

	// Not due yet → skipped even though pending.
	runOnce(t, r)
	assert.Empty(t, binder.delivered("orders"))

	// Fast-forward the retry time, attempt 2 fails again.
	store.mu.Lock()
	for _, row := range store.rows {
		row.nextRetry = time.Now()
	}
	store.mu.Unlock()
	runOnce(t, r)
	assert.Empty(t, binder.delivered("orders"))

	// Attempt 3 succeeds.
	store.mu.Lock()
	for _, row := range store.rows {
		row.nextRetry = time.Now()
	}
	store.mu.Unlock()
	runOnce(t, r)
	assert.Len(t, binder.delivered("orders"), 1)
	assert.Len(t, store.Snapshot(StatusSent), 1)
	require.Len(t, retries, 2)
}

func TestRelay_DeadLettersAfterMaxAttempts(t *testing.T) {
	store := &MemoryStore{}
	binder := newMemBinder(map[string]int{"orders": 0}) // always fails
	store.Add(Record{Destination: "orders", Key: "k1", Payload: []byte("poison"),
		Headers: map[string]string{"h": "v"}}, time.Time{})

	var dead []*Record
	r := NewRelay(store, binder, Config{
		PollInterval: time.Hour, MaxAttempts: 3, BackoffBase: time.Millisecond, DLQSuffix: ".dlq",
	}, ObserverFunc{OnDeadFunc: func(rec *Record, err error) { dead = append(dead, rec) }})

	for i := 0; i < 3; i++ {
		store.mu.Lock()
		for _, row := range store.rows {
			row.nextRetry = time.Now()
		}
		store.mu.Unlock()
		runOnce(t, r)
	}

	assert.Empty(t, binder.delivered("orders"))
	dlq := binder.delivered("orders.dlq")
	require.Len(t, dlq, 1)
	assert.Equal(t, "poison", string(dlq[0].Payload))
	assert.Equal(t, "v", dlq[0].Headers["h"])
	assert.Equal(t, "k1", dlq[0].Headers[messaging.HeaderDLQKey])
	assert.Equal(t, "3", dlq[0].Headers[messaging.HeaderDLQRetries])
	assert.Contains(t, dlq[0].Headers[messaging.HeaderDLQError], "boom")
	assert.Len(t, store.Snapshot(StatusDead), 1)
	require.Len(t, dead, 1)
}

func TestRelay_DeadWithoutDLQSuffix(t *testing.T) {
	store := &MemoryStore{}
	binder := newMemBinder(map[string]int{"orders": 0})
	store.Add(Record{Destination: "orders", Payload: []byte("m1")}, time.Time{})

	r := NewRelay(store, binder, Config{
		PollInterval: time.Hour, MaxAttempts: 1, DLQSuffix: "",
	})
	runOnce(t, r)

	assert.Empty(t, binder.delivered("orders"))
	assert.Empty(t, binder.delivered("orders.dlq"))
	assert.Len(t, store.Snapshot(StatusDead), 1)
}

func TestRelay_DeadLetterPublishFailureKeepsPending(t *testing.T) {
	store := &MemoryStore{}
	// Destination fails AND its DLQ fails → record must stay pending.
	binder := newMemBinder(map[string]int{"orders": 0, "orders.dlq": 0})
	store.Add(Record{Destination: "orders", Payload: []byte("m1")}, time.Time{})

	r := NewRelay(store, binder, Config{
		PollInterval: time.Hour, MaxAttempts: 1, BackoffBase: time.Millisecond, DLQSuffix: ".dlq",
	})
	runOnce(t, r)

	assert.Empty(t, binder.delivered("orders.dlq"))
	assert.Len(t, store.Snapshot(StatusPending), 1, "record must stay pending when the DLQ copy fails")
	assert.Empty(t, store.Snapshot(StatusDead))
}

func TestRelay_PublishOrderingPerKeyPassedThrough(t *testing.T) {
	store := &MemoryStore{}
	binder := newMemBinder(nil)
	store.Add(Record{Destination: "orders", Key: "user-1", Payload: []byte("m1")}, time.Time{})

	r := NewRelay(store, binder, Config{PollInterval: time.Hour})
	runOnce(t, r)

	msgs := binder.delivered("orders")
	require.Len(t, msgs, 1)
	assert.Equal(t, "user-1", msgs[0].Key)
}

func TestRelay_RunStopsOnCancel(t *testing.T) {
	store := &MemoryStore{}
	binder := newMemBinder(nil)
	r := NewRelay(store, binder, Config{PollInterval: time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop on cancel")
	}
	require.NoError(t, r.Close())
}

func TestRelay_BatchSurvivesPoisonRecord(t *testing.T) {
	store := &MemoryStore{}
	// "poison" always fails; "healthy" succeeds — one bad row must not starve the other.
	binder := newMemBinder(map[string]int{"poison": 0})
	store.Add(Record{Destination: "poison", Payload: []byte("bad")}, time.Time{})
	store.Add(Record{Destination: "healthy", Payload: []byte("good")}, time.Time{})

	r := NewRelay(store, binder, Config{PollInterval: time.Hour, MaxAttempts: 1, BackoffBase: time.Millisecond, DLQSuffix: ".dlq"})
	runOnce(t, r)

	assert.Len(t, binder.delivered("healthy"), 1)
	assert.Len(t, store.Snapshot(StatusSent), 1)
	assert.Len(t, store.Snapshot(StatusDead), 1)
}

func TestConfig_BackoffDoublingAndCap(t *testing.T) {
	c := Config{BackoffBase: time.Second, BackoffMax: 5 * time.Second}.withDefaults()
	assert.Equal(t, time.Second, c.backoff(1))
	assert.Equal(t, 2*time.Second, c.backoff(2))
	assert.Equal(t, 4*time.Second, c.backoff(3))
	assert.Equal(t, 5*time.Second, c.backoff(4))
	assert.Equal(t, 5*time.Second, c.backoff(100))
}

func TestConfig_Defaults(t *testing.T) {
	c := Config{}.withDefaults()
	assert.Equal(t, time.Second, c.PollInterval)
	assert.Equal(t, 100, c.BatchSize)
	assert.Equal(t, 8, c.MaxAttempts)
	assert.Equal(t, time.Second, c.BackoffBase)
	assert.Equal(t, time.Minute, c.BackoffMax)
	assert.Equal(t, "", c.DLQSuffix)
}
