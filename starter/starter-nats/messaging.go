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

package StarterNats

import (
	"context"

	"github.com/nats-io/nats.go"
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/propagate"
)

// NewDriver adapts a NATS connection to the broker-neutral messaging.Driver, so
// application code can publish/consume messaging.Message envelopes without
// depending on the nats API. destination/source strings are NATS subjects; the
// subscriber group maps onto a NATS queue group (competing consumers). The raw
// *Conn bean stays available for JetStream and other NATS-specific features this
// driver does not model.
//
// Trace context rides the envelope: the messaging.Observe decorator wraps this
// driver, injecting the current W3C context into the message headers on
// publish and extracting it on consume, so a trace links producer to consumer
// across services. The driver therefore calls the raw *nats.Conn (not the
// instrumented Conn.PublishMsgContext/Conn.Consume): the messaging path is
// observed once, by Observe, and the raw path by the Conn wrapper. All tracing
// is a no-op without starter-otel.
//
// prop is the load-test convention the driver carries: publish stamps the
// marker onto the NATS message header and consume reads it back. A nil
// propagator falls back to [traffic.NewDefaultPropagator].
func NewDriver(conn *Conn, prop traffic.Propagator) messaging.Driver {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return messaging.Observe(&driver{conn: conn, prop: prop}, "nats")
}

type driver struct {
	conn *Conn
	prop traffic.Propagator
}

func (b *driver) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	return &publisher{conn: b.conn, subject: destination, prop: b.prop}, nil
}

func (b *driver) NewSubscriber(_ context.Context, source, group string) (messaging.Subscriber, error) {
	return &subscriber{conn: b.conn, subject: source, group: group, prop: b.prop}, nil
}

// publisher sends envelopes to a fixed NATS subject.
type publisher struct {
	conn    *Conn
	subject string
	prop    traffic.Propagator
}

// headerMsgKey carries the envelope's ordering key across NATS: core NATS
// subjects have no key concept, so the driver round-trips msg.Key through a
// reserved message header and restores it on consume.
const headerMsgKey = "x-msg-key"

func (p *publisher) Publish(ctx context.Context, msg *messaging.Message) error {
	messaging.EnsureMessageID(msg)
	nm := &nats.Msg{Subject: p.subject, Data: msg.Payload, Header: toNatsHeader(msg.Headers)}
	if msg.Key != "" {
		if nm.Header == nil {
			nm.Header = nats.Header{}
		}
		nm.Header.Set(headerMsgKey, msg.Key)
	}
	// Carry the load-test marker in the NATS message header so the consumer
	// recognises synthetic load. The header map is allocated on demand because a
	// headerless message leaves it nil and writing to it must not panic; the
	// adapter writes through Header.Set, so the key lands in canonical spelling.
	if p.prop.IsLoadTest(ctx) {
		if nm.Header == nil {
			nm.Header = nats.Header{}
		}
		p.prop.Inject(ctx, propagate.Header(nm.Header))
	}
	// Publish through the raw *nats.Conn: the Observe decorator already opened
	// the producer span, injected the W3C trace context into msg.Headers (which
	// toNatsHeader copied into nm.Header) and took the metrics; going through
	// Conn.PublishMsgContext would count and span every message a second time.
	// nats.go's core PublishMsg carries no context parameter, which is fine —
	// the span's lifetime is Observe's, and nothing raw here needs the ctx.
	return p.conn.Conn.PublishMsg(nm)
}

func (p *publisher) Close() error { return nil }

// subscriber delivers messages from a fixed NATS subject to a handler. When
// group is non-empty it joins a queue group so only one member of the group
// receives each message.
type subscriber struct {
	conn    *Conn
	subject string
	group   string
	prop    traffic.Propagator
	sub     *nats.Subscription
}

func (s *subscriber) Subscribe(ctx context.Context, handler messaging.Handler) error {
	// Recover converts a handler panic into the normal error path
	// (nack/redelivery) instead of unwinding into the SDK goroutine. The error
	// it returns is recorded by the Observe decorator wrapping this handler.
	handler = messaging.Recover(handler)
	// Subscribe through the raw *nats.Conn: the Observe decorator owns the
	// consume span + metric + access log and extracts the upstream trace from
	// the envelope headers; going through Conn.Consume would count and span
	// every message a second time.
	cb := func(nm *nats.Msg) {
		octx := context.Background()
		// Extract the load-test marker the producer put in the NATS header.
		octx = s.prop.Extract(octx, propagate.Header(nm.Header))
		_ = handler(octx, fromNatsMsg(nm)) // core NATS delivery is fire-and-forget
	}
	var sub *nats.Subscription
	var err error
	if s.group != "" {
		sub, err = s.conn.Conn.QueueSubscribe(s.subject, s.group, cb)
	} else {
		sub, err = s.conn.Conn.Subscribe(s.subject, cb)
	}
	if err != nil {
		return err
	}
	s.sub = sub
	return nil
}

func (s *subscriber) Close() error {
	if s.sub == nil {
		return nil
	}
	return s.sub.Unsubscribe()
}

// toNatsHeader converts the envelope headers into a nats.Header. It returns nil
// for an empty map so a plain Publish path is unaffected.
func toNatsHeader(h map[string]string) nats.Header {
	if len(h) == 0 {
		return nil
	}
	nh := make(nats.Header, len(h))
	for k, v := range h {
		nh.Set(k, v)
	}
	return nh
}

// fromNatsMsg builds a messaging.Message from a received nats.Msg. The envelope
// header form is single-valued, so a multi-valued NATS header keeps only its
// first value; the transport-internal x-msg-key header is lifted into Message.Key
// rather than exposed as an application header.
func fromNatsMsg(nm *nats.Msg) *messaging.Message {
	var headers map[string]string
	if len(nm.Header) > 0 {
		headers = make(map[string]string, len(nm.Header))
		for k := range nm.Header {
			headers[k] = nm.Header.Get(k)
		}
	}
	key := nm.Header.Get(headerMsgKey)
	delete(headers, headerMsgKey)
	return &messaging.Message{Key: key, Payload: nm.Data, Headers: headers}
}
