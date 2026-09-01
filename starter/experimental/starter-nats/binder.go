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
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/governance/traffic"
)

// NewBinder adapts a NATS connection to the broker-neutral messaging.Binder, so
// application code can publish/consume messaging.Message envelopes without
// depending on the nats API. destination/source strings are NATS subjects; the
// subscriber group maps onto a NATS queue group (competing consumers). The raw
// *Conn bean stays available for JetStream and other NATS-specific features this
// binder does not model.
//
// Trace context rides the envelope: publish injects the current W3C context into
// the message header via StartPublishSpan and consume extracts it via
// StartConsumeSpan, so a trace links producer to consumer across services. All
// tracing is a no-op without starter-otel.
func NewBinder(conn *Conn) messaging.Binder {
	return &binder{conn: conn}
}

type binder struct{ conn *Conn }

func (b *binder) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	return &publisher{conn: b.conn, subject: destination}, nil
}

func (b *binder) NewSubscriber(_ context.Context, source, group string) (messaging.Subscriber, error) {
	return &subscriber{conn: b.conn, subject: source, group: group}, nil
}

// publisher sends envelopes to a fixed NATS subject.
type publisher struct {
	conn    *Conn
	subject string
}

// headerMsgKey carries the envelope's ordering key across NATS: core NATS
// subjects have no key concept, so the binder round-trips msg.Key through a
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
	// recognises synthetic load. NATS header Set canonicalises like net/http.
	if traffic.IsLoadTest(ctx) {
		if nm.Header == nil {
			nm.Header = nats.Header{}
		}
		nm.Header.Set(traffic.HeaderLoadTest, "1")
	}
	// Conn.PublishMsg emits the producer span + metric + access log and injects
	// the W3C trace context into nm.Header for subscribers.
	return p.conn.PublishMsg(nm)
}

func (p *publisher) Close() error { return nil }

// subscriber delivers messages from a fixed NATS subject to a handler. When
// group is non-empty it joins a queue group so only one member of the group
// receives each message.
type subscriber struct {
	conn    *Conn
	subject string
	group   string
	sub     *nats.Subscription
}

func (s *subscriber) Subscribe(_ context.Context, handler messaging.Handler) error {
	// Recover converts a handler panic into the normal error path
	// (nack/redelivery) instead of unwinding into the SDK goroutine.
	handler = messaging.Recover(handler)
	cb := func(nm *nats.Msg) {
		// startConsume extracts the upstream trace, opens a consumer span +
		// metric + access log; nil-safe when observability is off.
		ctx, sp := s.conn.startConsume(context.Background(), s.subject, nm)
		// Extract the load-test marker the producer put in the NATS header.
		if traffic.IsAffirmative(nm.Header.Get(traffic.HeaderLoadTest)) {
			ctx = traffic.WithLoadTest(ctx, "nats-header")
		}
		err := handler(ctx, fromNatsMsg(nm))
		if sp != nil {
			sp.End(err)
		}
	}
	var (
		sub *nats.Subscription
		err error
	)
	if s.group != "" {
		sub, err = s.conn.QueueSubscribe(s.subject, s.group, cb)
	} else {
		sub, err = s.conn.Subscribe(s.subject, cb)
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
