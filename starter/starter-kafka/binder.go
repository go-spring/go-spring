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

package StarterKafka

import (
	"context"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/log"
	"go-spring.org/stdlib/messaging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// NewBinder adapts a franz-go Kafka client to the broker-neutral
// messaging.Binder, so application code can publish/consume messaging.Message
// envelopes without depending on the kgo API. The raw *kgo.Client bean stays
// available for transactions, admin and other Kafka-specific features this
// binder does not model.
//
// Two Kafka realities shape the mapping:
//   - Publish is fully general: destination is the target topic and the message
//     is produced synchronously so a produce error is returned to the caller.
//   - Consume is constrained by franz-go: a client's consumer topics and group
//     are fixed at client construction (ConsumeTopics/ConsumerGroup, i.e. the
//     starter's topic/group config), and one client is a single consumer. So the
//     subscriber polls the client's configured topics; its source argument
//     selects among them by topic name and group is taken from the client
//     config. Use one client bean per logical consumer.
//
// Trace context rides the envelope: publish injects the current W3C context into
// record headers and consume extracts it before invoking the handler, so a trace
// links producer to consumer across services (a no-op without an OTel
// propagator). Per-request client spans are already emitted by the kotel hooks
// installed on the client.
func NewBinder(cl *kgo.Client) messaging.Binder {
	return &binder{cl: cl}
}

type binder struct{ cl *kgo.Client }

func (b *binder) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	return &publisher{cl: b.cl, topic: destination}, nil
}

func (b *binder) NewSubscriber(_ context.Context, source, _ string) (messaging.Subscriber, error) {
	return &subscriber{cl: b.cl, topic: source}, nil
}

// publisher produces envelopes to a fixed topic.
type publisher struct {
	cl    *kgo.Client
	topic string
}

func (p *publisher) Publish(ctx context.Context, msg *messaging.Message) error {
	rec := &kgo.Record{
		Topic:   p.topic,
		Value:   msg.Payload,
		Headers: toRecordHeaders(msg.Headers),
	}
	if msg.Key != "" {
		rec.Key = []byte(msg.Key)
	}
	otel.GetTextMapPropagator().Inject(ctx, recordCarrier{rec})
	return p.cl.ProduceSync(ctx, rec).FirstErr()
}

func (p *publisher) Close() error { return nil }

// subscriber delivers records from the client's configured topics to a handler,
// filtered to a single topic. It runs one background poll loop that stops when
// Close cancels its context.
type subscriber struct {
	cl     *kgo.Client
	topic  string
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
			fetches := s.cl.PollFetches(loopCtx)
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, e := range errs {
					if e.Err != context.Canceled {
						log.Errorf(loopCtx, log.TagAppDef, "kafka binder poll error on %q: %v", e.Topic, e.Err)
					}
				}
			}
			fetches.EachRecord(func(rec *kgo.Record) {
				if s.topic != "" && rec.Topic != s.topic {
					return
				}
				msgCtx := otel.GetTextMapPropagator().Extract(loopCtx, recordCarrier{rec})
				if err := handler(msgCtx, fromRecord(rec)); err != nil {
					log.Errorf(msgCtx, log.TagAppDef, "kafka binder handler error on %q: %v", rec.Topic, err)
				}
			})
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

// toRecordHeaders converts envelope headers into kgo record headers, returning
// nil for an empty map.
func toRecordHeaders(h map[string]string) []kgo.RecordHeader {
	if len(h) == 0 {
		return nil
	}
	hs := make([]kgo.RecordHeader, 0, len(h))
	for k, v := range h {
		hs = append(hs, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}
	return hs
}

// fromRecord builds a messaging.Message from a consumed kgo record.
func fromRecord(rec *kgo.Record) *messaging.Message {
	var headers map[string]string
	if len(rec.Headers) > 0 {
		headers = make(map[string]string, len(rec.Headers))
		for _, h := range rec.Headers {
			headers[h.Key] = string(h.Value)
		}
	}
	return &messaging.Message{
		Key:       string(rec.Key),
		Payload:   rec.Value,
		Headers:   headers,
		Timestamp: rec.Timestamp,
	}
}

// recordCarrier adapts a kgo.Record's headers to the OTel TextMapCarrier
// interface for trace-context injection (publish) and extraction (consume).
type recordCarrier struct{ rec *kgo.Record }

func (c recordCarrier) Get(key string) string {
	for _, h := range c.rec.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c recordCarrier) Set(key, value string) {
	for i := range c.rec.Headers {
		if c.rec.Headers[i].Key == key {
			c.rec.Headers[i].Value = []byte(value)
			return
		}
	}
	c.rec.Headers = append(c.rec.Headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
}

func (c recordCarrier) Keys() []string {
	keys := make([]string, 0, len(c.rec.Headers))
	for _, h := range c.rec.Headers {
		keys = append(keys, h.Key)
	}
	return keys
}

var _ propagation.TextMapCarrier = recordCarrier{}
