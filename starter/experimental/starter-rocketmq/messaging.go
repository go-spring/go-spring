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

package StarterRocketmq

import (
	"context"
	"strconv"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/propagate"
	"go-spring.org/log"
)

// NewDriver adapts a RocketMQ client to the broker-neutral messaging.Driver,
// so application code can publish/consume messaging.Message envelopes without
// depending on the RocketMQ API. destination/source strings are RocketMQ
// topics; the subscriber group maps onto a RocketMQ consumer group
// (clustering mode, i.e. competing consumers within the group). The raw
// Client bean stays available for pull consumers, orderly consumption,
// transactions and other RocketMQ-specific features this driver does not
// model.
//
// A publisher owns one started producer and a subscriber owns one started
// push consumer; both are registered on the Client and released on Close (and
// again by the client's own teardown, where Shutdown is idempotent enough to
// be safe).
//
// Trace context rides the envelope: the messaging.Observe decorator wraps this
// driver, injecting the current W3C context into the message headers on
// publish (mapped onto RocketMQ user properties) and extracting it on consume,
// so a trace links producer to consumer across services. It also supplies the
// spans, metrics and access log. All of it is a no-op without starter-otel.
//
// prop is the process's load-test convention (nullable — nil falls back to
// traffic.NewDefaultPropagator), carried on every producer/consumer this driver
// hands out.
func NewDriver(cl *Client, prop traffic.Propagator) messaging.Driver {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return messaging.Observe(&driver{cl: cl, prop: prop}, "rocketmq")
}

type driver struct {
	cl   *Client
	prop traffic.Propagator
}

func (b *driver) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	p, err := b.cl.NewProducer(producer.WithGroupName("go-spring-" + destination))
	if err != nil {
		return nil, err
	}
	return &publisher{p: p, topic: destination, prop: b.prop}, nil
}

func (b *driver) NewSubscriber(_ context.Context, source, group string) (messaging.Subscriber, error) {
	// A RocketMQ consumer must belong to a group; when none is given we derive
	// a stable one from the topic so a lone consumer still works.
	g := group
	if g == "" {
		g = "go-spring-" + source
	}
	c, err := b.cl.NewPushConsumer(consumer.WithGroupName(g))
	if err != nil {
		return nil, err
	}
	return &subscriber{cl: b.cl, c: c, topic: source, prop: b.prop}, nil
}

// publisher produces envelopes to a fixed topic via its own producer.
type publisher struct {
	p     rocketmq.Producer
	topic string
	prop  traffic.Propagator
}

func (p *publisher) Publish(ctx context.Context, msg *messaging.Message) error {
	messaging.EnsureMessageID(msg)
	m := primitive.NewMessage(p.topic, msg.Payload)
	if msg.Key != "" {
		m.WithKeys([]string{msg.Key})
	}
	for k, v := range msg.Headers {
		m.WithProperty(k, v)
	}
	// Carry the load-test marker (if any) in the user properties so the
	// consumer can recognise synthetic load. A message is not a map, so the
	// written entries go through its own property setter.
	c := propagate.StringMap{}
	p.prop.Inject(ctx, c)
	for k, v := range c {
		m.WithProperty(k, v)
	}
	_, err := p.p.SendSync(ctx, m)
	return err
}

func (p *publisher) Close() error {
	return p.p.Shutdown()
}

// subscriber delivers messages from a fixed topic/group to a handler. Delivery
// runs on the SDK's push-consumer goroutines; a handler error asks RocketMQ to
// redeliver the message (ConsumeRetryLater), success acknowledges it.
type subscriber struct {
	cl    *Client
	c     rocketmq.PushConsumer
	topic string
	prop  traffic.Propagator
}

func (s *subscriber) Subscribe(_ context.Context, handler messaging.Handler) error {
	// Recover converts a handler panic into the normal error path
	// (nack/redelivery) instead of unwinding into the SDK goroutine.
	handler = messaging.Recover(handler)
	// Subscribe must be called before Start: the SDK builds its subscription
	// data from the Subscribe calls, then Start kicks off rebalancing.
	err := s.c.Subscribe(s.topic, consumer.MessageSelector{
		Type:       consumer.TAG,
		Expression: "*",
	}, func(ctx context.Context, exts ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, ext := range exts {
			// Extract the load-test marker the producer put in the user
			// properties so the handler sees synthetic load via the
			// propagator's IsLoadTest(ctx).
			ctx = s.prop.Extract(ctx, propagate.StringMap(ext.GetProperties()))
			herr := handler(ctx, fromMessageExt(ext))
			if herr != nil {
				log.Errorf(ctx, log.TagAppDef, "rocketmq driver handler error on %q: %v", ext.Topic, herr)
				return consumer.ConsumeRetryLater, herr
			}
		}
		return consumer.ConsumeSuccess, nil
	})
	if err != nil {
		return err
	}
	return s.c.Start()
}

func (s *subscriber) Close() error {
	return s.c.Shutdown()
}

// fromMessageExt builds a messaging.Message from a received MessageExt.
func fromMessageExt(ext *primitive.MessageExt) *messaging.Message {
	m := &messaging.Message{
		Key:       ext.GetProperty(primitive.PropertyKeys),
		Payload:   ext.Body,
		Headers:   ext.GetProperties(),
		Timestamp: time.UnixMilli(ext.StoreTimestamp),
	}
	// Surface the broker's redelivery count as the reserved header so the
	// handler can decide retry vs dead-letter without in-process state.
	m.SetHeader(messaging.HeaderDeliveryAttempt, strconv.FormatInt(int64(ext.ReconsumeTimes)+1, 10))
	return m
}
