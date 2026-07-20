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
// for publish/subscribe messaging — the Spring Cloud Stream "binder" equivalent
// expressed in Go idioms.
//
// It lets application code publish and consume [Message] envelopes through a
// uniform [Publisher] / [Subscriber] pair, so switching the underlying broker
// (NATS, Kafka, ...) is a wiring change rather than a business-code rewrite. A
// broker starter supplies a [Binder] that opens publishers/subscribers against
// a concrete connection; the raw client bean stays available as an escape hatch
// for broker-specific features this abstraction deliberately does not model.
//
// Observability rides the envelope: because Headers is a plain map[string]string
// it doubles as a W3C trace-context carrier, so a binder injects trace context
// on publish and extracts it on consume without this package importing any
// tracing library.
package messaging

import (
	"context"
	"time"
)

// Message is the broker-neutral envelope carried across a binder. Payload is the
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

	// Timestamp is when the message was produced. Binders set it on consume from
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
// the binder that delivery failed; how that is surfaced (nack, redelivery, log)
// is broker-specific and documented by each binder.
type Handler func(ctx context.Context, msg *Message) error

// Publisher sends messages to a single destination bound at creation time. It is
// obtained from [Binder.NewPublisher]. Implementations must be safe for
// concurrent use; Close releases the underlying producer resources (it does not
// close the shared client bean the binder was built from).
type Publisher interface {
	// Publish sends msg to the bound destination.
	Publish(ctx context.Context, msg *Message) error

	// Close releases resources held by this publisher.
	Close() error
}

// Subscriber consumes messages from a single source bound at creation time. It
// is obtained from [Binder.NewSubscriber]. Subscribe starts delivery to handler
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

// Binder opens publishers and subscribers against one broker connection. A
// broker starter implements it over its native client (e.g. *nats.Conn,
// *kgo.Client) and hands the Binder to the application; the destination/source
// strings are interpreted in the broker's own terms (subject, topic, ...).
type Binder interface {
	// NewPublisher returns a Publisher bound to destination.
	NewPublisher(ctx context.Context, destination string) (Publisher, error)

	// NewSubscriber returns a Subscriber bound to source. group is the optional
	// competing-consumer group (queue group / consumer group); empty means each
	// subscriber receives every message (broadcast), where the broker supports it.
	NewSubscriber(ctx context.Context, source, group string) (Subscriber, error)
}
