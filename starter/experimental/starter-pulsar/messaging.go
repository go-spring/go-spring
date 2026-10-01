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

package StarterPulsar

import (
	"context"
	"maps"
	"sync"

	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/propagate"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
)

// NewDriver adapts a Pulsar client to the broker-neutral messaging.Driver, so
// application code can publish/consume messaging.Message envelopes without
// depending on the pulsar API. destination/source strings are Pulsar topics;
// the subscriber group maps onto a Pulsar subscription name (a shared
// subscription, i.e. competing consumers). The raw pulsar.Client bean stays
// available for readers, the admin API, schemas and other Pulsar-specific
// features this driver does not model.
//
// A publisher owns one pulsar.Producer for its topic and a subscriber owns one
// pulsar.Consumer for its subscription; both are created lazily by the driver
// and released on Close.
//
// Each publish and consume DECLARES its operation (see [operation]) and runs it
// under the client's resilience executor, which is the single emitter of the
// span, the metrics and the access log — pulsar-client-go exposes no
// reject-capable middleware, so the executor is driven at the call site. The
// declared layer also injects/extracts the W3C trace context through the
// message Properties (mapped onto the envelope headers), so a trace links
// producer to consumer across services. All of it is a no-op without
// starter-otel.
//
// prop is the process's load-test convention (nullable — nil falls back to
// traffic.NewDefaultPropagator), carried on every producer/consumer this driver
// hands out.
func NewDriver(cl pulsar.Client, prop traffic.Propagator) messaging.Driver {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return &driver{cl: cl, prop: prop}
}

type driver struct {
	cl   pulsar.Client
	prop traffic.Propagator
}

func (b *driver) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	p, err := b.cl.CreateProducer(pulsar.ProducerOptions{Topic: destination})
	if err != nil {
		return nil, err
	}
	return &publisher{cl: b.cl, p: p, prop: b.prop}, nil
}

func (b *driver) NewSubscriber(_ context.Context, source, group string) (messaging.Subscriber, error) {
	// A Pulsar consumer must name a subscription; when no group is given we
	// derive a stable one from the topic so a lone consumer still works.
	sub := group
	if sub == "" {
		sub = "go-spring-" + source
	}
	c, err := b.cl.Subscribe(pulsar.ConsumerOptions{
		Topic:            source,
		SubscriptionName: sub,
		Type:             pulsar.Shared,
	})
	if err != nil {
		return nil, err
	}
	return &subscriber{cl: b.cl, c: c, source: source, prop: b.prop}, nil
}

// publisher produces envelopes to a fixed topic via its own producer. It holds
// the owning client so Publish can resolve the client-scoped resilience
// executor (producers are caller-created and carry no stable identity).
type publisher struct {
	cl   pulsar.Client
	p    pulsar.Producer
	prop traffic.Propagator
}

func (p *publisher) Publish(ctx context.Context, msg *messaging.Message) error {
	messaging.EnsureMessageID(msg)
	// Carry the load-test marker (if any) in the message Properties so the
	// consumer can recognise synthetic load. The properties map is copied when
	// the marker is written rather than mutating the caller's, to avoid
	// surprising the publisher.
	props := msg.Headers
	c := propagate.StringMap{}
	p.prop.Inject(ctx, c)
	if len(c) > 0 {
		cp := make(map[string]string, len(msg.Headers)+len(c))
		maps.Copy(cp, msg.Headers)
		maps.Copy(cp, c)
		props = cp
	}
	pm := &pulsar.ProducerMessage{
		Payload:    msg.Payload,
		Properties: props,
	}
	if msg.Key != "" {
		pm.Key = msg.Key
	}
	// Route through the same seam the raw client API uses (GuardedSend):
	// GuardedSend declares the publish (topic/direction) and runs it under the
	// client-scoped resilience executor, a no-op pass-through when governance is
	// off for this client and a rejection sentinel when rate-limited/circuit-open.
	_, err := GuardedSend(ctx, p.cl, p.p, pm)
	return err
}

func (p *publisher) Close() error {
	p.p.Close() // pulsar Producer.Close has no error return
	return nil
}

// subscriber delivers messages from a fixed topic/subscription to a handler. It
// runs one background receive loop that stops when Close cancels its context. A
// handler error nacks the message so Pulsar can redeliver it; success acks it.
//
// cl is held so each delivery can declare its consume and resolve the
// client-scoped resilience executor (the emitter); source is the topic the
// subscription was opened on, used as the declared destination.
type subscriber struct {
	cl     pulsar.Client
	c      pulsar.Consumer
	source string
	prop   traffic.Propagator
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func (s *subscriber) Subscribe(ctx context.Context, handler messaging.Handler) error {
	// Recover converts a handler panic into the normal error path
	// (nack/redelivery) instead of unwinding into the SDK goroutine.
	handler = messaging.Recover(handler)
	loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.cancel = cancel
	s.done = make(chan struct{})

	go func() {
		defer close(s.done)
		for loopCtx.Err() == nil {
			msg, err := s.c.Receive(loopCtx)
			if err != nil {
				if loopCtx.Err() != nil {
					return // context cancelled by Close
				}
				log.Errorf(loopCtx, log.TagAppDef, "pulsar driver receive error: %v", err)
				continue
			}
			// Extract the load-test marker the producer put in Properties so the
			// handler sees synthetic load via the propagator's IsLoadTest(msgCtx),
			// then continue the producer's W3C trace and declare the consume. The
			// handler runs under the resilience executor, which emits the span,
			// metrics and access log from the declaration.
			msgCtx := s.prop.Extract(loopCtx, propagate.StringMap(msg.Properties()))
			msgCtx = extractTraceContext(msgCtx, msg.Properties())
			msgCtx = observability.WithOperation(msgCtx, operation(opConsume, s.source))
			herr := guard(msgCtx, s.cl, func(attemptCtx context.Context) error {
				return handler(attemptCtx, fromPulsarMsg(msg))
			})
			if herr != nil {
				log.Errorf(msgCtx, log.TagAppDef, "pulsar driver handler error on %q: %v", msg.Topic(), herr)
				s.c.Nack(msg)
			} else if err := s.c.Ack(msg); err != nil {
				// A failed ack means the broker will redeliver the message.
				log.Warnf(msgCtx, log.TagAppDef, "pulsar: ack failed on %q, message may be redelivered: %v", msg.Topic(), err)
			}
		}
	}()
	return nil
}

func (s *subscriber) Close() error {
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.done != nil {
			<-s.done
		}
		s.c.Close()
	})
	return nil
}

// fromPulsarMsg builds a messaging.Message from a received pulsar.Message.
func fromPulsarMsg(msg pulsar.Message) *messaging.Message {
	return &messaging.Message{
		Key:       msg.Key(),
		Payload:   msg.Payload(),
		Headers:   msg.Properties(),
		Timestamp: msg.PublishTime(),
	}
}
