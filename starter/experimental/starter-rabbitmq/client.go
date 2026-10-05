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
// broker-neutral messaging.Driver adapter wrapping an *amqp.Connection, plus the
// publisher/subscriber entities it produces and the header<->envelope
// conversion helpers. The raw *amqp.Connection bean stays available for
// exchanges, custom routing, publisher confirms and other AMQP features this
// driver does not model.

package StarterRabbitMQ

import (
	"context"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/propagate"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
)

// NewDriver adapts a RabbitMQ client to the broker-neutral messaging.Driver,
// so application code can publish/consume messaging.Message envelopes without
// depending on the amqp API. destination/source strings are queue names: a
// publisher sends to the default exchange keyed by the queue name, and a
// subscriber consumes from the named queue. Competing consumers arise naturally
// when several subscribers share a queue, so the group argument is unused (a
// RabbitMQ queue already *is* the consumer group). The raw *amqp.Connection
// bean stays available for exchanges, custom routing, publisher confirms and
// other AMQP features this driver does not model.
//
// Each publisher and subscriber owns its own AMQP channel (channels are not
// safe for concurrent use), opened by the driver and closed on Close. Both
// declare the queue idempotently so a round trip works without external setup.
//
// The driver DECLARES each operation's identity (see [operation]) and routes it
// through the connection's resilience executor — the single emitter of the span,
// the durations and the access log. It does not instrument anything itself:
// publish rides [GuardedPublish], and the consume handler runs under [Client.execute]
// after the upstream W3C context is extracted from the delivery headers, so a
// trace links producer to consumer across services. All of it is a no-op
// without starter-otel.
//
// prop is the process's load-test convention (nullable — nil falls back to
// traffic.NewDefaultPropagator), carried on every publisher/subscriber this
// driver hands out.
func NewDriver(cl *Client, prop traffic.Propagator) messaging.Driver {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return &driver{cl: cl, prop: prop}
}

type driver struct {
	cl   *Client
	prop traffic.Propagator
}

func (b *driver) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	ch, err := b.cl.Conn.Channel()
	if err != nil {
		return nil, err
	}
	if _, err := ch.QueueDeclare(destination, false, false, false, false, nil); err != nil {
		_ = ch.Close()
		return nil, err
	}
	return &publisher{cl: b.cl, ch: ch, queue: destination, prop: b.prop}, nil
}

func (b *driver) NewSubscriber(_ context.Context, source, _ string) (messaging.Subscriber, error) {
	ch, err := b.cl.Conn.Channel()
	if err != nil {
		return nil, err
	}
	if _, err := ch.QueueDeclare(source, false, false, false, false, nil); err != nil {
		_ = ch.Close()
		return nil, err
	}
	return &subscriber{cl: b.cl, ch: ch, queue: source, prop: b.prop}, nil
}

// publisher sends envelopes to a fixed queue via the default exchange. It holds
// the owning connection so Publish can resolve the connection-scoped resilience
// executor (channels carry no identity of their own).
type publisher struct {
	cl    *Client
	ch    *amqp.Channel
	queue string
	prop  traffic.Propagator
}

func (p *publisher) Publish(ctx context.Context, msg *messaging.Message) error {
	messaging.EnsureMessageID(msg)
	pub := amqp.Publishing{
		Body:      msg.Payload,
		Headers:   toAMQPTable(msg.Headers),
		MessageId: msg.Key,
	}
	// Carry the load-test marker in the AMQP headers so the consumer recognises
	// synthetic load. toAMQPTable yields nil for an empty header map, so the
	// table is allocated on demand and the written entries go into it as
	// strings.
	c := propagate.StringMap{}
	p.prop.Inject(ctx, c)
	for k, v := range c {
		if pub.Headers == nil {
			pub.Headers = amqp.Table{}
		}
		pub.Headers[k] = v
	}
	// Route through the same resilience executor the raw client API uses
	// (GuardedPublish): it declares the publish's identity and injects the W3C
	// trace context, then runs the send under the connection's executor — a
	// no-op pass-through when governance is off for this connection, a rejection
	// sentinel when rate-limited/circuit-open.
	return GuardedPublish(ctx, p.cl, p.ch, "", p.queue, false, false, pub)
}

func (p *publisher) Close() error { return p.ch.Close() }

// subscriber delivers messages from a fixed queue to a handler. It runs one
// background loop over the delivery channel that stops when Close closes the
// channel (which closes the delivery channel). A handler error nacks with
// requeue so the broker can redeliver; success acks.
//
// It holds the owning connection so Subscribe can resolve the connection-scoped
// resilience executor (channels carry no identity of their own).
type subscriber struct {
	cl    *Client
	ch    *amqp.Channel
	queue string
	prop  traffic.Propagator
	done  chan struct{}
	once  sync.Once
}

func (s *subscriber) Subscribe(_ context.Context, handler messaging.Handler) error {
	// Recover converts a handler panic into the normal error path
	// (nack/redelivery) instead of unwinding into the SDK goroutine.
	handler = messaging.Recover(handler)
	deliveries, err := s.ch.Consume(s.queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		for d := range deliveries {
			// Extract the upstream W3C trace into a fresh ctx first, then declare
			// the consume's identity, so the executor's consumer span nests under
			// the producer's rather than starting a new trace root. The load-test
			// marker the producer put in the AMQP headers is re-entered into the
			// same ctx.
			msgCtx := extractW3C(context.Background(), &d)
			msgCtx = s.prop.Extract(msgCtx, tableCarrier(d.Headers))
			msgCtx = observability.WithOperation(msgCtx, operation(opConsume, s.queue))
			// The handler runs under the connection's resilience executor, which
			// emits the consume span, durations and access log; without an
			// executor guard runs it inline.
			herr := s.cl.execute(msgCtx, func(attemptCtx context.Context) error {
				return handler(attemptCtx, fromDelivery(&d))
			})
			if herr != nil {
				log.Error(msgCtx, log.TagAppDef, herr, log.String("queue", s.queue), log.Msg("rabbitmq driver handler failed"))
				if err := d.Nack(false, true); err != nil {
					log.Warn(msgCtx, log.TagAppDef, log.String("queue", s.queue), log.Err(err), log.Msg("rabbitmq: nack failed, message may be redelivered"))
				}
			} else if err := d.Ack(false); err != nil {
				// A failed ack means the broker will redeliver the message.
				log.Warn(msgCtx, log.TagAppDef, log.String("queue", s.queue), log.Err(err), log.Msg("rabbitmq: ack failed, message may be redelivered"))
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
	m := &messaging.Message{
		Key:       d.MessageId,
		Payload:   d.Body,
		Headers:   headers,
		Timestamp: d.Timestamp,
	}
	// AMQP exposes only a redelivered flag (no count): surface it as the
	// reserved header, "2" meaning "at least the second delivery".
	if d.Redelivered {
		m.SetHeader(messaging.HeaderDeliveryAttempt, "2")
	}
	return m
}

// tableCarrier adapts an AMQP table to [propagate.Carrier]: only string
// values read — non-string table entries are not the marker's shape. A view:
// Set writes through.
type tableCarrier amqp.Table

var _ propagate.Carrier = tableCarrier(nil)

// Keys returns the table's keys, in no particular order.
func (t tableCarrier) Keys() []string {
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	return keys
}

// Values returns the string value stored under key, nil otherwise.
func (t tableCarrier) Values(key string) []string {
	if v, ok := t[key].(string); ok && v != "" {
		return []string{v}
	}
	return nil
}

// Set stores value under key as a string, replacing.
func (t tableCarrier) Set(key, value string) {
	t[key] = value
}
