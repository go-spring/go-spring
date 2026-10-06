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

// messaging.go adapts the raw *sarama.Client to the broker-neutral
// messaging.Driver, mirroring starter-kafka (franz-go). The raw client bean
// stays available for the transaction, admin and other Kafka-specific features
// this adapter does not model; the guarded produce seam and the per-message
// consume seam it rides live in command.go.

package StarterKafkaSarama

import (
	"context"
	"sync"

	"github.com/IBM/sarama"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// NewDriver adapts a sarama client to the broker-neutral messaging.Driver, so
// application code can publish/consume messaging.Message envelopes without
// depending on the sarama API. Assert the raw client bean to sarama.Client and
// pass it here; the returned Driver is registered as a bean of the same name by
// the starter (see starter.go).
//
// Two Kafka realities shape the mapping, and they differ from franz-go's:
//   - Publish is fully general: destination is the target topic; one Publisher
//     owns one SyncProducer derived from the client, so a produce error is
//     returned to the caller. The send rides [WrapSyncProducer], so it is
//     declared, traced and governance-guarded on the same seam an application
//     using the raw producer wraps with.
//   - Consume is bound at Subscribe time, not at client construction: sarama's
//     client carries no topics or group, so source names the topic and group
//     names the consumer group (both required). One Subscriber owns one
//     ConsumerGroup. A handler error leaves the message unmarked, so it is
//     redelivered on the next rebalance / restart; this adapter does not force an
//     immediate rewind. The delivery rides [Consume], the same per-message seam
//     an application owning its own consumer group calls.
//
// Trace context rides the envelope: the publish seam injects the W3C context
// into the record headers and the consume seam extracts it before invoking the
// handler, so a trace links producer to consumer where both sides use these
// seams. sarama's send API takes no context, so a publish's span is a new root
// (see [WrapSyncProducer]); the load-test marker is carried the same way.
//
// prop is the load-test convention the driver carries; a nil propagator falls
// back to [traffic.NewDefaultPropagator].
func NewDriver(cl sarama.Client, prop traffic.Propagator) messaging.Driver {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return &driver{cl: cl, prop: prop}
}

type driver struct {
	cl   sarama.Client
	prop traffic.Propagator
}

func (d *driver) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	p, err := sarama.NewSyncProducerFromClient(d.cl)
	if err != nil {
		return nil, errutil.Explain(err, "kafka-sarama: create sync producer for %q failed", destination)
	}
	return &publisher{prod: WrapSyncProducer(d.cl, p, d.prop), topic: destination}, nil
}

func (d *driver) NewSubscriber(_ context.Context, source, group string) (messaging.Subscriber, error) {
	if group == "" {
		return nil, errutil.Explain(nil, "kafka-sarama: subscriber for %q needs a consumer group", source)
	}
	return &subscriber{cl: d.cl, topic: source, group: group, prop: d.prop}, nil
}

// publisher produces envelopes to a fixed topic through one guarded SyncProducer.
type publisher struct {
	prod  sarama.SyncProducer
	topic string
}

func (p *publisher) Publish(_ context.Context, msg *messaging.Message) error {
	messaging.EnsureMessageID(msg)
	pm := &sarama.ProducerMessage{
		Topic:   p.topic,
		Value:   sarama.ByteEncoder(msg.Payload),
		Headers: toProducerHeaders(msg.Headers),
	}
	if msg.Key != "" {
		pm.Key = sarama.StringEncoder(msg.Key)
	}
	// WrapSyncProducer injects the trace context and the load-test marker and
	// routes the send through the client's resilience executor — a no-op
	// pass-through when governance is off, a rejection sentinel when
	// rate-limited / circuit-open.
	_, _, err := p.prod.SendMessage(pm)
	return err
}

func (p *publisher) Close() error { return p.prod.Close() }

// subscriber delivers records from one topic to a handler through a consumer
// group. It runs one background consume loop that stops when Close cancels its
// context. Each delivery is declared and run under the client's resilience
// executor (via [Consume]), which emits its span, metrics and access log; with
// governance off the executor is a pass-through and the handler runs inline.
type subscriber struct {
	cl     sarama.Client
	topic  string
	group  string
	prop   traffic.Propagator
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func (s *subscriber) Subscribe(ctx context.Context, handler messaging.Handler) error {
	// Recover converts a handler panic into the normal error path
	// (unmarked → redelivery) instead of unwinding into the consumer goroutine.
	handler = messaging.Recover(handler)
	cg, err := sarama.NewConsumerGroupFromClient(s.group, s.cl)
	if err != nil {
		return errutil.Explain(err, "kafka-sarama: create consumer group %q failed", s.group)
	}
	loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.cancel = cancel
	s.done = make(chan struct{})

	h := &groupHandler{cl: s.cl, prop: s.prop, handler: handler}
	go func() {
		defer close(s.done)
		defer func() {
			if cerr := cg.Close(); cerr != nil {
				log.Warn(loopCtx, log.TagAppDef, log.String("topic", s.topic), log.Msg("close kafka-sarama consumer group failed"))
			}
		}()
		for loopCtx.Err() == nil {
			if err := cg.Consume(loopCtx, []string{s.topic}, h); err != nil && loopCtx.Err() == nil {
				log.Error(loopCtx, log.TagAppDef, err, log.String("topic", s.topic), log.Msg("kafka-sarama consumer group failed"))
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
	})
	return nil
}

// groupHandler drives each claimed partition, running every record through
// [Consume] and marking it only on success.
type groupHandler struct {
	cl      sarama.Client
	prop    traffic.Propagator
	handler messaging.Handler
}

func (h *groupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *groupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *groupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			err := Consume(session.Context(), h.cl, msg, h.prop, func(ctx context.Context) error {
				return h.handler(ctx, fromConsumerMessage(msg))
			})
			if err != nil {
				log.Error(session.Context(), log.TagAppDef, err, log.String("topic", msg.Topic), log.Msg("kafka-sarama driver handler failed"))
				// Leave the record unmarked so it is redelivered on the next
				// rebalance / restart rather than silently dropped.
				continue
			}
			session.MarkMessage(msg, "")
		}
	}
}

// toProducerHeaders converts envelope headers into sarama record headers,
// returning nil for an empty map.
func toProducerHeaders(h map[string]string) []sarama.RecordHeader {
	if len(h) == 0 {
		return nil
	}
	hs := make([]sarama.RecordHeader, 0, len(h))
	for k, v := range h {
		hs = append(hs, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
	}
	return hs
}

// fromConsumerMessage builds a messaging.Message from a consumed sarama record.
func fromConsumerMessage(msg *sarama.ConsumerMessage) *messaging.Message {
	var headers map[string]string
	if len(msg.Headers) > 0 {
		headers = make(map[string]string, len(msg.Headers))
		for _, h := range msg.Headers {
			headers[string(h.Key)] = string(h.Value)
		}
	}
	return &messaging.Message{
		Key:       string(msg.Key),
		Payload:   msg.Value,
		Headers:   headers,
		Timestamp: msg.Timestamp,
	}
}
