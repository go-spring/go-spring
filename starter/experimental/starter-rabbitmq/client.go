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

// client.go is the "resource entity" concept of this starter: the
// broker-neutral messaging.Binder adapter wrapping an *amqp.Connection, plus the
// publisher/subscriber entities it produces and the header<->envelope
// conversion helpers. The raw *amqp.Connection bean stays available for
// exchanges, custom routing, publisher confirms and other AMQP features this
// binder does not model.
package StarterRabbitMQ

import (
	"context"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/cloud/experimental/messaging"
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/log"
)

// NewBinder adapts a RabbitMQ connection to the broker-neutral messaging.Binder,
// so application code can publish/consume messaging.Message envelopes without
// depending on the amqp API. destination/source strings are queue names: a
// publisher sends to the default exchange keyed by the queue name, and a
// subscriber consumes from the named queue. Competing consumers arise naturally
// when several subscribers share a queue, so the group argument is unused (a
// RabbitMQ queue already *is* the consumer group). The raw *amqp.Connection
// bean stays available for exchanges, custom routing, publisher confirms and
// other AMQP features this binder does not model.
//
// Each publisher and subscriber owns its own AMQP channel (channels are not
// safe for concurrent use), opened by the binder and closed on Close. Both
// declare the queue idempotently so a round trip works without external setup.
//
// Trace context rides the envelope: publish injects the current W3C context into
// the message headers via StartPublishSpan and consume extracts it via
// StartConsumeSpan, so a trace links producer to consumer across services. All
// tracing is a no-op without starter-otel.
func NewBinder(conn *amqp.Connection) messaging.Binder {
	return &binder{conn: conn}
}

type binder struct{ conn *amqp.Connection }

func (b *binder) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	ch, err := b.conn.Channel()
	if err != nil {
		return nil, err
	}
	if _, err := ch.QueueDeclare(destination, false, false, false, false, nil); err != nil {
		_ = ch.Close()
		return nil, err
	}
	return &publisher{conn: b.conn, ch: ch, queue: destination}, nil
}

func (b *binder) NewSubscriber(_ context.Context, source, _ string) (messaging.Subscriber, error) {
	ch, err := b.conn.Channel()
	if err != nil {
		return nil, err
	}
	if _, err := ch.QueueDeclare(source, false, false, false, false, nil); err != nil {
		_ = ch.Close()
		return nil, err
	}
	return &subscriber{ch: ch, queue: source}, nil
}

// publisher sends envelopes to a fixed queue via the default exchange. It holds
// the owning connection so Publish can resolve the connection-scoped resilience
// executor (channels carry no identity of their own).
type publisher struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	queue string
}

func (p *publisher) Publish(ctx context.Context, msg *messaging.Message) error {
	pub := amqp.Publishing{
		Body:      msg.Payload,
		Headers:   toAMQPTable(msg.Headers),
		MessageId: msg.Key,
	}
	// Carry the load-test marker in the AMQP headers so the consumer recognises
	// synthetic load.
	if traffic.IsLoadTest(ctx) {
		if pub.Headers == nil {
			pub.Headers = amqp.Table{}
		}
		pub.Headers[traffic.MetaKeyLoadTest] = "1"
	}
	ctx, sp := startPublish(ctx, p.queue, &pub)
	// Route through the same resilience executor the raw client API uses
	// (GuardedPublish): a no-op pass-through when governance is off for this
	// connection, a rejection sentinel when rate-limited/circuit-open.
	err := GuardedPublish(ctx, p.conn, p.ch, "", p.queue, false, false, pub)
	sp.End(err)
	return err
}

func (p *publisher) Close() error { return p.ch.Close() }

// subscriber delivers messages from a fixed queue to a handler. It runs one
// background loop over the delivery channel that stops when Close closes the
// channel (which closes the delivery channel). A handler error nacks with
// requeue so the broker can redeliver; success acks.
type subscriber struct {
	ch    *amqp.Channel
	queue string
	done  chan struct{}
	once  sync.Once
}

func (s *subscriber) Subscribe(_ context.Context, handler messaging.Handler) error {
	// SafeHandler converts a handler panic into the normal error path
	// (nack/redelivery) instead of unwinding into the SDK goroutine.
	handler = messaging.SafeHandler(handler)
	deliveries, err := s.ch.Consume(s.queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		for d := range deliveries {
			msgCtx, sp := startConsume(context.Background(), &d)
			// Extract the load-test marker the producer put in the AMQP headers.
			if v, ok := d.Headers[traffic.MetaKeyLoadTest].(string); ok && traffic.IsAffirmative(v) {
				msgCtx = traffic.WithLoadTest(msgCtx, "amqp-header")
			}
			herr := handler(msgCtx, fromDelivery(&d))
			sp.End(herr)
			if herr != nil {
				log.Errorf(msgCtx, log.TagAppDef, "rabbitmq binder handler error on %q: %v", s.queue, herr)
				if err := d.Nack(false, true); err != nil {
					log.Warnf(msgCtx, log.TagAppDef, "rabbitmq: nack failed on %q, message may be redelivered: %v", s.queue, err)
				}
			} else if err := d.Ack(false); err != nil {
				// A failed ack means the broker will redeliver the message.
				log.Warnf(msgCtx, log.TagAppDef, "rabbitmq: ack failed on %q, message may be redelivered: %v", s.queue, err)
			}
		}
	}()
	return nil
}

func (s *subscriber) Close() error {
	var err error
	s.once.Do(func() {
		err = s.ch.Close()
		if s.done != nil {
			<-s.done
		}
	})
	return err
}

// toAMQPTable converts envelope headers into an amqp.Table, returning nil for an
// empty map.
func toAMQPTable(h map[string]string) amqp.Table {
	if len(h) == 0 {
		return nil
	}
	t := make(amqp.Table, len(h))
	for k, v := range h {
		t[k] = v
	}
	return t
}

// fromDelivery builds a messaging.Message from a received amqp.Delivery,
// flattening the string-valued headers into the envelope form.
func fromDelivery(d *amqp.Delivery) *messaging.Message {
	var headers map[string]string
	if len(d.Headers) > 0 {
		headers = make(map[string]string, len(d.Headers))
		for k, v := range d.Headers {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}
	return &messaging.Message{
		Key:       d.MessageId,
		Payload:   d.Body,
		Headers:   headers,
		Timestamp: d.Timestamp,
	}
}
