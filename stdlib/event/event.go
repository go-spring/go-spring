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

// Package event provides the Go-idiomatic equivalent of Spring's
// ApplicationEventPublisher / @EventListener: an in-process publish/subscribe
// bus that lets modules communicate by publishing typed events instead of
// calling each other directly. A configuration-change event, a bean-ready
// event, or any custom domain event is published once and delivered to every
// interested handler, so producers and consumers stay decoupled.
//
// It deliberately does not replicate the Spring mechanism. There is no marker
// interface an event must implement, no annotation scanning, and no reflection
// over handler signatures. An event is any ordinary Go value — idiomatically a
// concrete struct — and subscription is a plain generic function that keeps the
// call site type-safe. The only reflection is a single type key used to route a
// published value to the handlers registered for that exact type, mirroring the
// restraint of [go-spring.org/stdlib/aspect] (the type-safe interceptor chain)
// and [go-spring.org/stdlib/lock] (the interface-as-seam abstraction).
//
// This is strictly an in-process bus. It is not a cross-instance broadcast: for
// fan-out across replicas over a message queue, that is a separate concern
// (config bus) and does not overlap with this package.
//
// # Publishing and subscribing
//
// A [Bus] delivers a published value to the handlers subscribed for its dynamic
// type. Subscription is a free generic function so handlers stay strongly typed:
//
//	type ConfigChanged struct{ Key, Value string }
//
//	bus := event.New()
//	cancel := event.Subscribe(bus, func(ctx context.Context, e ConfigChanged) error {
//	    log.Printf("reload %s=%s", e.Key, e.Value)
//	    return nil
//	})
//	defer cancel()
//
//	err := bus.Publish(ctx, ConfigChanged{Key: "log.level", Value: "debug"})
//
// Routing is by exact dynamic type: publishing a ConfigChanged reaches handlers
// subscribed as ConfigChanged, not handlers subscribed to some interface it
// satisfies. This keeps delivery predictable and reflection-free at the call
// site; use a concrete struct per event, the Go-idiomatic choice.
//
// # Synchronous vs asynchronous
//
// [Subscribe] runs the handler inline on the publishing goroutine, in
// deterministic order (see [WithOrder]); its error is aggregated into the value
// returned by [Bus.Publish]. [SubscribeAsync] hands the event to a dedicated
// worker goroutine through a buffered channel and returns immediately, so a slow
// handler never stalls the publisher. An async handler's error cannot flow back
// to Publish; route it with [WithErrorHandler] instead. [Bus.Close] gracefully
// stops every async worker after draining events already buffered.
//
// # Error aggregation
//
// A synchronous handler that fails does not stop the others: every handler runs
// and their errors are combined with [errors.Join], so one faulty subscriber
// cannot silently suppress the rest. This follows the pass-through spirit of the
// aspect chain — the bus stays out of the way and reports what happened.
//
// # nil / empty transparency
//
// Publishing an event with no subscribers is a no-op that returns nil, and
// subscribing on a nil bus returns a no-op cancel. Wiring the bus therefore
// stays inert until a producer and a consumer actually meet, the same
// transparent-pass-through property the aspect and resilience packages rely on.
//
// # Container integration
//
// A bean can subscribe declaratively by implementing [Listener]: the container
// collects every bean exported as a Listener (the same "match by Export" pattern
// [go-spring.org/stdlib/health.Indicator] uses) and a small registrar calls
// [Listener.Register] on each once the bus and the listeners are constructed,
// keeping the container's core untouched. See the example module.
package event

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
)

// ErrClosed is returned by [Bus.Publish] once [Bus.Close] has been called. A
// closed bus delivers to no one; publishing to it is a programming error rather
// than the benign no-listener case, so it is reported explicitly instead of
// being silently dropped.
var ErrClosed = errors.New("event: bus is closed")

// Bus is an in-process publish/subscribe event bus. Implementations must be safe
// for concurrent use: multiple goroutines may publish and subscribe at once.
type Bus interface {
	// Publish delivers event to every handler subscribed for its dynamic type.
	// Synchronous handlers run inline in deterministic order and their errors are
	// combined with errors.Join; asynchronous handlers are enqueued for their
	// worker and do not contribute to the returned error. A nil event or a type
	// with no subscribers is a no-op returning nil. After Close it returns
	// [ErrClosed].
	Publish(ctx context.Context, event any) error

	// Close stops the bus. It signals every asynchronous worker to finish,
	// letting each drain the events already buffered before exiting, and blocks
	// until they have all stopped. It is idempotent.
	Close() error
}

// Listener is the optional seam for declarative, container-driven subscription.
// A bean exported as Listener is collected by the container and its Register is
// called with the shared bus, inside which it wires its handlers via [Subscribe]
// / [SubscribeAsync] — so the type-safe generic subscription happens at the
// listener, while the container only needs the non-generic Listener contract to
// collect beans (mirroring how health.Indicator is collected by Export).
type Listener interface {
	// Register subscribes this listener's handlers onto bus. It is called once,
	// after the bus and all listener beans have been constructed.
	Register(bus Bus)
}

// subOptions holds the normalized options for a single subscription.
type subOptions struct {
	order   int
	buffer  int
	onError func(ctx context.Context, err error)
}

// SubOption customizes a single subscription created by [Subscribe] or
// [SubscribeAsync].
type SubOption func(*subOptions)

// WithOrder sets the delivery priority of a synchronous handler: lower values
// run first (index 0 is outermost, matching the aspect chain's ordering
// convention). Handlers with equal order keep registration order. It has no
// effect on asynchronous handlers, whose workers run independently.
func WithOrder(order int) SubOption {
	return func(o *subOptions) { o.order = order }
}

// WithBuffer sets the capacity of an asynchronous subscription's event channel.
// A larger buffer absorbs bigger bursts before Publish must wait for the worker
// to catch up. It defaults to [DefaultBuffer] and is ignored by synchronous
// subscriptions.
func WithBuffer(n int) SubOption {
	return func(o *subOptions) { o.buffer = n }
}

// WithErrorHandler routes the error returned by an asynchronous handler, which
// cannot propagate back to [Bus.Publish]. Without it, an async handler's error
// is discarded. It is ignored by synchronous subscriptions, whose errors are
// aggregated into Publish's return value.
func WithErrorHandler(fn func(ctx context.Context, err error)) SubOption {
	return func(o *subOptions) { o.onError = fn }
}

// DefaultBuffer is the channel capacity of an asynchronous subscription when
// [WithBuffer] is not given.
const DefaultBuffer = 64

// New returns a new in-process [Bus].
func New() Bus {
	return &bus{subs: make(map[reflect.Type][]*entry)}
}

// asyncEvent carries a published value and its context to an async worker.
type asyncEvent struct {
	ctx   context.Context
	event any
}

// entry is one subscription registered on the bus.
type entry struct {
	order  int
	seq    uint64
	async  bool
	invoke func(ctx context.Context, event any) error

	// Async-only machinery. ch carries events to the worker; done is closed to
	// tell the worker to drain and exit. Senders select on done as well, so once
	// it is closed a send never blocks — avoiding both a leaked publisher and a
	// send on a closed channel.
	ch       chan asyncEvent
	done     chan struct{}
	stopOnce sync.Once
}

// stop signals the worker to finish. It is safe to call more than once.
func (e *entry) stop() {
	e.stopOnce.Do(func() { close(e.done) })
}

// bus is the default in-process [Bus] implementation.
type bus struct {
	mu     sync.RWMutex
	seq    uint64
	subs   map[reflect.Type][]*entry
	closed bool
	wg     sync.WaitGroup
}

// subscribable is implemented only by *bus, so the generic subscription
// functions can reach the concrete registrar without exporting it.
type subscribable interface {
	subscribe(typ reflect.Type, invoke func(ctx context.Context, event any) error, async bool, o subOptions) func()
}

// subscribe registers one subscription and returns a cancel function. The cancel
// is idempotent and, for an async subscription, stops its worker.
func (b *bus) subscribe(typ reflect.Type, invoke func(ctx context.Context, event any) error, async bool, o subOptions) func() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return func() {}
	}
	b.seq++
	e := &entry{order: o.order, seq: b.seq, async: async, invoke: invoke}
	if async {
		buf := o.buffer
		if buf <= 0 {
			buf = DefaultBuffer
		}
		e.ch = make(chan asyncEvent, buf)
		e.done = make(chan struct{})
		b.wg.Add(1)
		go b.runWorker(e, o.onError)
	}
	list := append(b.subs[typ], e)
	// Keep the slice sorted so Publish delivers in ascending order, ties broken
	// by registration sequence for determinism.
	slices.SortStableFunc(list, func(a, c *entry) int {
		if a.order != c.order {
			return a.order - c.order
		}
		return int(a.seq) - int(c.seq)
	})
	b.subs[typ] = list
	b.mu.Unlock()

	var once sync.Once
	return func() { once.Do(func() { b.remove(typ, e) }) }
}

// remove detaches an entry and, if async, stops its worker.
func (b *bus) remove(typ reflect.Type, e *entry) {
	b.mu.Lock()
	list := b.subs[typ]
	if i := slices.Index(list, e); i >= 0 {
		b.subs[typ] = slices.Delete(list, i, i+1)
		if len(b.subs[typ]) == 0 {
			delete(b.subs, typ)
		}
	}
	b.mu.Unlock()
	if e.async {
		e.stop()
	}
}

// runWorker consumes an async subscription's events until told to stop, then
// drains any events already buffered so a graceful Close does not silently drop
// work that was already accepted.
func (b *bus) runWorker(e *entry, onError func(ctx context.Context, err error)) {
	defer b.wg.Done()
	deliver := func(ae asyncEvent) {
		if err := e.invoke(ae.ctx, ae.event); err != nil && onError != nil {
			onError(ae.ctx, err)
		}
	}
	for {
		select {
		case ae := <-e.ch:
			deliver(ae)
		case <-e.done:
			for {
				select {
				case ae := <-e.ch:
					deliver(ae)
				default:
					return
				}
			}
		}
	}
}

// Publish implements [Bus].
func (b *bus) Publish(ctx context.Context, event any) error {
	if event == nil {
		return nil
	}
	typ := reflect.TypeOf(event)

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrClosed
	}
	// Snapshot the subscriber list so handlers run without holding the lock,
	// letting subscribe/cancel proceed concurrently.
	snapshot := slices.Clone(b.subs[typ])
	b.mu.RUnlock()

	var errs []error
	for _, e := range snapshot {
		if e.async {
			select {
			case e.ch <- asyncEvent{ctx: ctx, event: event}:
			case <-e.done:
				// Subscription was cancelled concurrently; skip it.
			case <-ctx.Done():
				errs = append(errs, ctx.Err())
			}
			continue
		}
		if err := e.invoke(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Close implements [Bus].
func (b *bus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	var entries []*entry
	for _, list := range b.subs {
		entries = append(entries, list...)
	}
	b.subs = make(map[reflect.Type][]*entry)
	b.mu.Unlock()

	for _, e := range entries {
		if e.async {
			e.stop()
		}
	}
	b.wg.Wait()
	return nil
}

// Subscribe registers a synchronous handler for events of type T. It runs inline
// on the publishing goroutine and its error is aggregated into [Bus.Publish]'s
// result. The returned cancel function removes the subscription; it is
// idempotent. Subscribing on a nil bus (or with a nil handler) returns a no-op
// cancel.
func Subscribe[T any](bus Bus, handler func(ctx context.Context, event T) error, opts ...SubOption) func() {
	return subscribe(bus, handler, false, opts)
}

// SubscribeAsync registers an asynchronous handler for events of type T. Each
// published event is delivered on a dedicated worker goroutine, so a slow
// handler never blocks the publisher; use [WithBuffer] to size its queue and
// [WithErrorHandler] to observe failures. The returned cancel stops the worker
// and is idempotent.
func SubscribeAsync[T any](bus Bus, handler func(ctx context.Context, event T) error, opts ...SubOption) func() {
	return subscribe(bus, handler, true, opts)
}

// subscribe is the shared body of [Subscribe] and [SubscribeAsync]: it boxes the
// typed handler behind an any-taking invoker (asserting T back on delivery) and
// registers it under T's type key.
func subscribe[T any](bus Bus, handler func(ctx context.Context, event T) error, async bool, opts []SubOption) func() {
	s, ok := bus.(subscribable)
	if !ok || handler == nil {
		return func() {}
	}
	var o subOptions
	for _, fn := range opts {
		fn(&o)
	}
	invoke := func(ctx context.Context, event any) error {
		// The router keys by T's type, so this assertion always holds; the guard
		// is defensive against a future non-standard bus implementation.
		v, ok := event.(T)
		if !ok {
			return nil
		}
		return handler(ctx, v)
	}
	return s.subscribe(reflect.TypeFor[T](), invoke, async, o)
}
