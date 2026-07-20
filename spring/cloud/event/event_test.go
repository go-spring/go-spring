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

package event_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/spring/cloud/event"
	"go-spring.org/stdlib/testing/assert"
)

// eventA and eventB are two distinct concrete event types used to verify that
// delivery is routed by exact dynamic type.
type eventA struct{ n int }
type eventB struct{ s string }

func TestPublishNoSubscribersIsNoOp(t *testing.T) {
	bus := event.New()
	// No subscribers of any kind: Publish returns nil and does nothing.
	err := bus.Publish(context.Background(), eventA{n: 1})
	assert.Error(t, err).Nil()

	// A nil event is likewise a no-op.
	err = bus.Publish(context.Background(), nil)
	assert.Error(t, err).Nil()
}

func TestSubscribeOnNilBusReturnsNoopCancel(t *testing.T) {
	// Subscribing on a nil bus must not panic and must yield a callable cancel.
	cancel := event.Subscribe(nil, func(context.Context, eventA) error { return nil })
	assert.That(t, cancel).NotNil()
	cancel()
}

func TestSyncDeliveryRoutedByType(t *testing.T) {
	bus := event.New()
	var gotA []int
	var gotB []string

	event.Subscribe(bus, func(_ context.Context, e eventA) error {
		gotA = append(gotA, e.n)
		return nil
	})
	event.Subscribe(bus, func(_ context.Context, e eventB) error {
		gotB = append(gotB, e.s)
		return nil
	})

	assert.Error(t, bus.Publish(context.Background(), eventA{n: 7})).Nil()
	assert.Error(t, bus.Publish(context.Background(), eventB{s: "hi"})).Nil()

	// Each handler receives only events of its own type.
	assert.Slice(t, gotA).Equal([]int{7})
	assert.Slice(t, gotB).Equal([]string{"hi"})
}

func TestSyncErrorsAreJoinedWithoutShortCircuit(t *testing.T) {
	bus := event.New()
	err1 := errors.New("boom-1")
	err2 := errors.New("boom-2")

	var ran int
	event.Subscribe(bus, func(context.Context, eventA) error { ran++; return err1 })
	event.Subscribe(bus, func(context.Context, eventA) error { ran++; return nil })
	event.Subscribe(bus, func(context.Context, eventA) error { ran++; return err2 })

	err := bus.Publish(context.Background(), eventA{})

	// Every handler ran despite earlier failures, and both errors surfaced.
	assert.That(t, ran).Equal(3)
	assert.Error(t, err).Is(err1)
	assert.Error(t, err).Is(err2)
}

func TestWithOrderControlsSyncDelivery(t *testing.T) {
	bus := event.New()
	var order []string

	event.Subscribe(bus, func(context.Context, eventA) error {
		order = append(order, "c")
		return nil
	}, event.WithOrder(10))
	event.Subscribe(bus, func(context.Context, eventA) error {
		order = append(order, "a")
		return nil
	}, event.WithOrder(-5))
	event.Subscribe(bus, func(context.Context, eventA) error {
		order = append(order, "b")
		return nil
	}) // default order 0, between the two

	assert.Error(t, bus.Publish(context.Background(), eventA{})).Nil()
	assert.Slice(t, order).Equal([]string{"a", "b", "c"})
}

func TestCancelRemovesSubscription(t *testing.T) {
	bus := event.New()
	var count int
	cancel := event.Subscribe(bus, func(context.Context, eventA) error {
		count++
		return nil
	})

	assert.Error(t, bus.Publish(context.Background(), eventA{})).Nil()
	assert.That(t, count).Equal(1)

	cancel()
	cancel() // idempotent

	assert.Error(t, bus.Publish(context.Background(), eventA{})).Nil()
	// No further deliveries after cancel.
	assert.That(t, count).Equal(1)
}

func TestAsyncDelivery(t *testing.T) {
	bus := event.New()
	var mu sync.Mutex
	var got []int
	done := make(chan struct{}, 3)

	event.SubscribeAsync(bus, func(_ context.Context, e eventA) error {
		mu.Lock()
		got = append(got, e.n)
		mu.Unlock()
		done <- struct{}{}
		return nil
	})

	for i := range 3 {
		assert.Error(t, bus.Publish(context.Background(), eventA{n: i})).Nil()
	}

	// Wait for all three async deliveries.
	for range 3 {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for async delivery")
		}
	}
	mu.Lock()
	assert.Slice(t, got).Equal([]int{0, 1, 2})
	mu.Unlock()
}

func TestAsyncErrorHandler(t *testing.T) {
	bus := event.New()
	boom := errors.New("async boom")
	got := make(chan error, 1)

	event.SubscribeAsync(bus, func(context.Context, eventA) error {
		return boom
	}, event.WithErrorHandler(func(_ context.Context, err error) {
		got <- err
	}))

	// The async handler error does not surface on Publish.
	assert.Error(t, bus.Publish(context.Background(), eventA{})).Nil()

	select {
	case err := <-got:
		assert.Error(t, err).Is(boom)
	case <-time.After(2 * time.Second):
		t.Fatal("error handler was not called")
	}
}

func TestCloseDrainsBufferedAsyncEvents(t *testing.T) {
	bus := event.New()
	var delivered atomic.Int64
	release := make(chan struct{})

	// A slow handler: the first event blocks in-flight while the rest queue up
	// in the buffer, so Close must drain them before returning.
	event.SubscribeAsync(bus, func(context.Context, eventA) error {
		<-release
		delivered.Add(1)
		return nil
	}, event.WithBuffer(16))

	const n = 5
	for i := range n {
		assert.Error(t, bus.Publish(context.Background(), eventA{n: i})).Nil()
	}

	// Let the worker proceed, then close: Close blocks until the worker has
	// drained every buffered event.
	close(release)
	assert.Error(t, bus.Close()).Nil()
	assert.That(t, int(delivered.Load())).Equal(n)

	// Publishing after Close is reported, not silently dropped.
	assert.Error(t, bus.Publish(context.Background(), eventA{})).Is(event.ErrClosed)
}

func TestCloseIsIdempotent(t *testing.T) {
	bus := event.New()
	event.SubscribeAsync(bus, func(context.Context, eventA) error { return nil })
	assert.Error(t, bus.Close()).Nil()
	assert.Error(t, bus.Close()).Nil()
}

func TestPublishHonorsContextCancellationForAsync(t *testing.T) {
	bus := event.New()
	block := make(chan struct{})
	defer close(block)

	// Buffer of 1 with a handler that never returns: after one event fills the
	// buffer and one is in-flight, the next Publish must respect ctx.
	event.SubscribeAsync(bus, func(context.Context, eventA) error {
		<-block
		return nil
	}, event.WithBuffer(1))

	assert.Error(t, bus.Publish(context.Background(), eventA{})).Nil() // in-flight
	assert.Error(t, bus.Publish(context.Background(), eventA{})).Nil() // buffered

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// The buffer is full and the worker is blocked, so this send would block;
	// the cancelled context makes Publish return its error instead.
	err := bus.Publish(ctx, eventA{})
	assert.Error(t, err).Is(context.Canceled)
}

// TestConcurrentPublishSubscribe exercises the bus under concurrent publishers,
// subscribers, and cancels so `go test -race` can flag data races.
func TestConcurrentPublishSubscribe(t *testing.T) {
	bus := event.New()
	defer bus.Close()

	var received atomic.Int64
	var wg sync.WaitGroup

	// Churn subscriptions concurrently.
	for range 8 {
		wg.Go(func() {
			for range 50 {
				cancel := event.Subscribe(bus, func(context.Context, eventA) error {
					received.Add(1)
					return nil
				})
				cancel()
			}
		})
	}

	// A stable async subscriber plus concurrent publishers.
	event.SubscribeAsync(bus, func(context.Context, eventA) error {
		received.Add(1)
		return nil
	})
	for range 8 {
		wg.Go(func() {
			for j := range 50 {
				_ = bus.Publish(context.Background(), eventA{n: j})
			}
		})
	}

	wg.Wait()
	// The exact count is nondeterministic due to interleaving; we only assert the
	// bus stayed live and the race detector found nothing.
	assert.That(t, received.Load() >= 0).True()
}
