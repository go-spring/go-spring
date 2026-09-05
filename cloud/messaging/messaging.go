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

// Package messaging defines a framework-agnostic, zero-dependency abstraction
// for publish/subscribe messaging, expressed in Go idioms.
//
// It lets application code publish and consume [Message] envelopes through a
// uniform [Publisher] / [Subscriber] pair, so switching the underlying broker
// (NATS, Kafka, ...) is a wiring change rather than a business-code rewrite. A
// broker starter supplies a [Driver] that opens publishers/subscribers against
// a concrete connection; the raw client bean stays available as an escape hatch
// for broker-specific features this abstraction deliberately does not model.
//
// Observability rides the envelope: because Headers is a plain map[string]string
// it doubles as a W3C trace-context carrier, so a driver injects trace context
// on publish and extracts it on consume without this package importing any
// tracing library.
package messaging

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Message is the broker-neutral envelope carried across a driver. Payload is the
// opaque body; Headers carries metadata (including propagated trace context);
// Key is the optional partitioning / ordering key that brokers with a notion of
// keyed streams (Kafka, JetStream) map onto their native key.
type Message struct {
	// Key is the optional partition/ordering key. Empty means unkeyed.
	Key string

	// Payload is the opaque message body.
	Payload []byte

	// Headers carries string metadata; it doubles as the trace-context carrier.
	// It may be nil on a freshly constructed message; use SetHeader to populate.
	Headers map[string]string

	// Timestamp is when the message was produced. Drivers set it on consume from
	// the broker's own timestamp when available; producers may leave it zero.
	Timestamp time.Time
}

// Header returns the value for key, or the empty string when absent.
func (m *Message) Header(key string) string {
	if m == nil || m.Headers == nil {
		return ""
	}
	return m.Headers[key]
}

// SetHeader sets key to value, allocating the map on first use.
func (m *Message) SetHeader(key, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
}

// Handler processes one consumed [Message]. Returning a non-nil error signals
// the driver that delivery failed; how that is surfaced (nack, redelivery, log)
// is broker-specific and documented by each driver.
type Handler func(ctx context.Context, msg *Message) error

// Publisher sends messages to a single destination bound at creation time. It is
// obtained from [Driver.NewPublisher]. Implementations must be safe for
// concurrent use; Close releases the underlying producer resources (it does not
// close the shared client bean the driver was built from).
type Publisher interface {
	// Publish sends msg to the bound destination.
	Publish(ctx context.Context, msg *Message) error

	// Close releases resources held by this publisher.
	Close() error
}

// Subscriber consumes messages from a single source bound at creation time. It
// is obtained from [Driver.NewSubscriber]. Subscribe starts delivery to handler
// and returns once delivery is established (it does not block); Close stops
// delivery and releases resources.
type Subscriber interface {
	// Subscribe starts delivering messages from the bound source to handler. It
	// returns after the subscription is established. Calling it more than once is
	// implementation-defined.
	Subscribe(ctx context.Context, handler Handler) error

	// Close stops delivery and releases resources held by this subscriber.
	Close() error
}

// Driver opens publishers and subscribers against one broker connection. A
// broker starter implements it over its native client (e.g. *nats.Conn,
// *kgo.Client) and hands the Driver to the application.
//
// The abstraction uses the neutral words "destination" (where a publisher
// sends) and "source" (where a subscriber listens); each driver interprets
// them as its broker's own address — Kafka topic, NATS subject, AMQP
// exchange/routing-key, RocketMQ topic, and so on. Callers write broker
// terms in their configuration; the driver maps them straight through.
type Driver interface {
	// NewPublisher returns a Publisher bound to destination.
	//
	// destination is the broker address to publish to (topic / subject /
	// exchange). It is resolved once here; the returned Publisher sends every
	// message to that same destination, and only there — one Publisher per
	// destination, one destination per Publisher.
	//
	// The returned Publisher is safe for concurrent use. ctx bounds the
	// setup work (connect, producer creation); it does not bound later
	// Publish calls, which take their own ctx.
	NewPublisher(ctx context.Context, destination string) (Publisher, error)

	// NewSubscriber returns a Subscriber bound to source.
	//
	// source is the broker address to consume from (topic / subject /
	// queue). group is the optional consumer group, interpreted with the
	// usual competing-consumer semantics: messages are delivered to only
	// one subscriber in the group (Kafka consumer group, NATS queue group,
	// RocketMQ consumer group), so replicas of the same service share the
	// load instead of each receiving everything. When group is empty, every
	// subscriber receives every message (broadcast / pub-sub fan-out), where
	// the broker supports it; group-bound ordering, if any, follows the
	// broker's rules (e.g. per Kafka partition) and is not modelled here.
	//
	// Delivery itself does not start until Subscribe is called on the
	// returned Subscriber. ctx bounds the setup work only.
	NewSubscriber(ctx context.Context, source, group string) (Subscriber, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Driver{}
)

// RegisterDriver makes a [Driver] available under name. It panics if name is
// empty, b is nil, or name is already registered, mirroring the driver-registry
// idiom used elsewhere (discovery.Register, resilience.RegisterDriver) so
// duplicate wiring fails loudly at init.
//
// Drivers are usually connection-bound and wired as beans via a starter's
// NewDriver constructor; this registry is the parity seam for applications that
// select a single process-wide driver by configured name.
func RegisterDriver(name string, b Driver) {
	if name == "" {
		panic("messaging: register with empty name")
	}
	if b == nil {
		panic("messaging: register nil driver for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		panic("messaging: driver already registered: " + name)
	}
	registry[name] = b
}

// GetDriver returns the [Driver] registered under name, or an error that lists
// the available drivers when none matches.
func GetDriver(name string) (Driver, error) {
	mu.RLock()
	defer mu.RUnlock()
	b, ok := registry[name]
	if !ok {
		names := make([]string, 0, len(registry))
		for k := range registry {
			names = append(names, k)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("messaging: no driver registered as %q (registered: %v)", name, names)
	}
	return b, nil
}
