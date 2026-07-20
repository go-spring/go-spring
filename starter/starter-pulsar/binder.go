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
	"sync"

	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/log"
	"go-spring.org/spring/cloud/messaging"
)

// NewBinder adapts a Pulsar client to the broker-neutral messaging.Binder, so
// application code can publish/consume messaging.Message envelopes without
// depending on the pulsar API. destination/source strings are Pulsar topics;
// the subscriber group maps onto a Pulsar subscription name (a shared
// subscription, i.e. competing consumers). The raw pulsar.Client bean stays
// available for readers, the admin API, schemas and other Pulsar-specific
// features this binder does not model.
//
// A publisher owns one pulsar.Producer for its topic and a subscriber owns one
// pulsar.Consumer for its subscription; both are created lazily by the binder
// and released on Close.
//
// Trace context rides the envelope: publish injects the current W3C context into
// the message Properties via StartProducerSpan and consume extracts it via
// StartConsumerSpan, so a trace links producer to consumer across services. All
// tracing is a no-op without starter-otel.
func NewBinder(cl pulsar.Client) messaging.Binder {
	return &binder{cl: cl}
}

type binder struct{ cl pulsar.Client }

func (b *binder) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	p, err := b.cl.CreateProducer(pulsar.ProducerOptions{Topic: destination})
	if err != nil {
		return nil, err
	}
	return &publisher{p: p}, nil
}

func (b *binder) NewSubscriber(_ context.Context, source, group string) (messaging.Subscriber, error) {
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
	return &subscriber{c: c}, nil
}

// publisher produces envelopes to a fixed topic via its own producer.
type publisher struct{ p pulsar.Producer }

func (p *publisher) Publish(ctx context.Context, msg *messaging.Message) error {
	pm := &pulsar.ProducerMessage{
		Payload:    msg.Payload,
		Properties: msg.Headers,
	}
	if msg.Key != "" {
		pm.Key = msg.Key
	}
	ctx, span := StartProducerSpan(ctx, pm)
	_, err := p.p.Send(ctx, pm)
	EndSpan(span, err)
	return err
}

func (p *publisher) Close() error {
	p.p.Close()
	return nil
}

// subscriber delivers messages from a fixed topic/subscription to a handler. It
// runs one background receive loop that stops when Close cancels its context. A
// handler error nacks the message so Pulsar can redeliver it; success acks it.
type subscriber struct {
	c      pulsar.Consumer
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func (s *subscriber) Subscribe(ctx context.Context, handler messaging.Handler) error {
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
				log.Errorf(loopCtx, log.TagAppDef, "pulsar binder receive error: %v", err)
				continue
			}
			msgCtx, span := StartConsumerSpan(loopCtx, msg)
			herr := handler(msgCtx, fromPulsarMsg(msg))
			EndSpan(span, herr)
			if herr != nil {
				log.Errorf(msgCtx, log.TagAppDef, "pulsar binder handler error on %q: %v", msg.Topic(), herr)
				s.c.Nack(msg)
			} else {
				_ = s.c.Ack(msg)
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
