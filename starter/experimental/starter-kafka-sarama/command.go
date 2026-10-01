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

// command.go is the "command seam" concept of this starter: the produce seam.
// It stacks two layers over a SyncProducer:
//
//	observe layer    — StartProducerSpan / StartConsumerSpan / EndSpan, the
//	                   call-site helpers (sarama.SendMessage carries no ctx)
//	resilience layer — WrapSyncProducer / guardedSyncProducer / applyResilience
//	                   / executorFor, indexed per client by sync.Map
package StarterKafkaSarama

import (
	"context"
	"sync"

	"github.com/IBM/sarama"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/cloud/propagate"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Why these are call-site helpers rather than a wrapped producer/consumer:
//
//  1. The only official OTel instrumentation for sarama, otelsarama
//     (go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/
//     otelsarama), is deprecated and still pinned to the abandoned
//     github.com/Shopify/sarama module. This starter uses github.com/IBM/sarama;
//     the two are distinct Go types, so otelsarama's WrapSyncProducer cannot wrap
//     an IBM producer, and importing it would drag in a second, conflicting
//     sarama fork. We therefore do the instrumentation natively on the OTel API.
//
//  2. sarama.SyncProducer.SendMessage takes no context.Context, so a producer
//     *wrapper* has nowhere to receive the request-scoped context from and could
//     only ever emit disconnected root spans. Passing ctx explicitly at the call
//     site is what makes distributed traces actually link across services.
//
// Everything here rides the OTel globals that starter-otel installs. Without
// starter-otel the global TracerProvider is a no-op and the global propagator is
// a no-op, so these helpers cost almost nothing and change no message bytes.

// Package-level observers back the helpers; kafka-sarama's helpers are the
// instrumentation API (there is no driver). They are built lazily (sync.Once)
// so a blank import of this starter pays no instrument construction at package
// init — only apps that actually call the helpers build them (see observe.go).
var (
	defaultObsOnce sync.Once
	defaultPubObs  *observer
	defaultSubObs  *observer
)

func pubObserver() *observer {
	defaultObsOnce.Do(func() {
		defaultPubObs = newObserver("publish", trace.SpanKindProducer)
		defaultSubObs = newObserver("consume", trace.SpanKindConsumer)
	})
	return defaultPubObs
}

func subObserver() *observer {
	defaultObsOnce.Do(func() {
		defaultPubObs = newObserver("publish", trace.SpanKindProducer)
		defaultSubObs = newObserver("consume", trace.SpanKindConsumer)
	})
	return defaultSubObs
}

// StartProducerSpan opens a producer observation for msg (span + duration/in-
// flight metric + access log) and injects the current W3C trace context into
// msg.Headers so downstream consumers can continue the trace. When ctx is
// load-test traffic the marker is stamped into msg.Headers too. Call it right
// before SyncProducer.SendMessage and End the returned span once the send
// completes:
//
//	_, span := StarterKafkaSarama.StartProducerSpan(ctx, msg, prop)
//	_, _, err := producer.SendMessage(msg)
//	StarterKafkaSarama.EndSpan(span, err)
//
// prop is the process's load-test convention; a nil propagator falls back to
// [traffic.NewDefaultPropagator].
func StartProducerSpan(ctx context.Context, msg *sarama.ProducerMessage, prop traffic.Propagator) (context.Context, *Span) {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	ctx, sp := pubObserver().Start(ctx, msg.Topic)
	otel.GetTextMapPropagator().Inject(ctx, producerCarrier{msg})
	// Carry the load-test marker in a record header so the consumer recognises
	// synthetic load. producerCarrier doubles as the traffic carrier: its Set is
	// idempotent, so re-injection never appends a second header.
	prop.Inject(ctx, producerCarrier{msg})
	return ctx, sp
}

// StartConsumerSpan extracts the upstream trace context carried in msg.Headers
// and opens a consumer observation, tagging the returned context as load-test
// traffic when msg carries the marker. Call it when a record is received and
// End once processing finishes:
//
//	_, span := StarterKafkaSarama.StartConsumerSpan(ctx, msg, prop)
//	err := handle(ctx, msg)
//	StarterKafkaSarama.EndSpan(span, err)
//
// prop is the process's load-test convention; a nil propagator falls back to
// [traffic.NewDefaultPropagator].
func StartConsumerSpan(ctx context.Context, msg *sarama.ConsumerMessage, prop traffic.Propagator) (context.Context, *Span) {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, consumerCarrier{msg})
	// Extract the load-test marker the producer put in a record header.
	ctx = prop.Extract(ctx, consumerCarrier{msg})
	return subObserver().Start(ctx, msg.Topic)
}

// EndSpan records err (if any) on the span and ends it.
func EndSpan(span *Span, err error) {
	span.End(err)
}

// producerCarrier adapts a sarama.ProducerMessage's headers to the OTel
// TextMapCarrier interface for context injection, and to propagate.Carrier
// for the load-test marker.
type producerCarrier struct{ msg *sarama.ProducerMessage }

var _ propagate.Carrier = producerCarrier{}

func (c producerCarrier) Get(key string) string {
	for _, h := range c.msg.Headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

// Values returns the single value Get finds, nil when absent or empty.
func (c producerCarrier) Values(key string) []string {
	if v := c.Get(key); v != "" {
		return []string{v}
	}
	return nil
}

func (c producerCarrier) Set(key, value string) {
	// Drop any existing header with the same key so re-injection stays idempotent.
	filtered := c.msg.Headers[:0]
	for _, h := range c.msg.Headers {
		if string(h.Key) != key {
			filtered = append(filtered, h)
		}
	}
	c.msg.Headers = append(filtered, sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
}

func (c producerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.msg.Headers))
	for _, h := range c.msg.Headers {
		keys = append(keys, string(h.Key))
	}
	return keys
}

// consumerCarrier adapts a sarama.ConsumerMessage's headers to the OTel
// TextMapCarrier interface for context extraction, and to propagate.Carrier
// for the load-test marker. Extraction never mutates the message, so Set is a
// no-op.
type consumerCarrier struct{ msg *sarama.ConsumerMessage }

var _ propagate.Carrier = consumerCarrier{}

func (c consumerCarrier) Get(key string) string {
	for _, h := range c.msg.Headers {
		if h != nil && string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

// Values returns the single value Get finds, nil when absent or empty.
func (c consumerCarrier) Values(key string) []string {
	if v := c.Get(key); v != "" {
		return []string{v}
	}
	return nil
}

func (c consumerCarrier) Set(string, string) {}

func (c consumerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.msg.Headers))
	for _, h := range c.msg.Headers {
		if h != nil {
			keys = append(keys, string(h.Key))
		}
	}
	return keys
}

var _ propagation.TextMapCarrier = producerCarrier{}
var _ propagation.TextMapCarrier = consumerCarrier{}

// clientGuard is the per-client resilience attachment: the executor chain and
// the stable service label it executes under, colocated so a guard lookup
// reads the pair atomically (no torn exec/service combination).
type clientGuard struct {
	exec    resilience.ClientExecutor
	service string
}

// clientGuards indexes the guard by the raw client bean, so WrapSyncProducer can resolve
// it from a bare sarama.Client and the destructor can Close it. Only clients with
// resilience enabled appear here.
var clientGuards sync.Map // sarama.Client -> *clientGuard

// applyResilience builds an executor and indexes it by client. This is the
// kafka-sarama seam of resilience: sarama exposes no reject-capable middleware,
// so the executor is driven through a transparent SyncProducer wrapper (see
// WrapSyncProducer) that callers opt into once after creating their producer.
//
// mgr and inj are the governance beans gs injects into the client constructor.
// A nil mgr is normalized here — an unarmed manager yields a transparent no-op
// executor, which is exactly "governance off", while a nil pointer would panic
// on the method call; inj is nil-safe at its use site, so a nil injector simply
// adds no fault.
func applyResilience(c Config, client sarama.Client, service string, mgr *resilience.Manager, inj *fault.Injector) error {
	exec := fault.WrapClientExecutor(mgr.ClientExecutorFor("kafka", service), service, inj)
	clientGuards.Store(client, &clientGuard{exec: exec, service: service})
	return nil
}

// closeResilience closes and forgets the executor behind client, if any.
func closeResilience(client sarama.Client) {
	if v, ok := clientGuards.LoadAndDelete(client); ok {
		_ = v.(*clientGuard).exec.Close()
	}
}

// executorFor loads the executor and service label attached to client. Returns
// (nil, "") when resilience is disabled for that client, so the wrapper falls
// back to a direct call.
func executorFor(client sarama.Client) (resilience.ClientExecutor, string) {
	v, ok := clientGuards.Load(client)
	if !ok {
		return nil, ""
	}
	g := v.(*clientGuard)
	return g.exec, g.service
}

// WrapSyncProducer returns a sarama.SyncProducer that routes SendMessage and
// SendMessages through the resilience executor attached to cl when governance
// is enabled. When governance is disabled (or cl was
// created without it) p is returned unchanged, so wrapping is a zero-risk
// opt-in: callers always wrap unconditionally.
//
// Only the synchronous send paths are guarded; the transaction and Close methods
// delegate directly, as does TxnStatus/IsTransactional. SendMessage takes no
// context.Context (sarama's API is context-free), so a background context is
// used — pass a per-call deadline via AttemptTimeout/MaxDuration in
// [resilience.Config] when a bound matters.
//
//	prod, _ := sarama.NewSyncProducerFromClient(cl)
//	prod = StarterKafkaSarama.WrapSyncProducer(cl, prod)
//	_, _, err := prod.SendMessage(msg) // now rate-limited / circuit-guarded
func WrapSyncProducer(cl sarama.Client, p sarama.SyncProducer) sarama.SyncProducer {
	exec, service := executorFor(cl)
	if exec == nil {
		return p
	}
	return &guardedSyncProducer{p: p, exec: exec, service: service}
}

// guardedSyncProducer is a transparent sarama.SyncProducer wrapper that drives
// the synchronous send methods through the resilience executor. All other
// methods (Close, transaction lifecycle, status queries) delegate to the inner
// producer unchanged — they are control-plane, not the protected data path.
type guardedSyncProducer struct {
	p       sarama.SyncProducer
	exec    resilience.ClientExecutor
	service string
}

var _ sarama.SyncProducer = (*guardedSyncProducer)(nil)

func (g *guardedSyncProducer) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	err = g.exec.Execute(context.Background(), func(context.Context) error {
		var perr error
		partition, offset, perr = g.p.SendMessage(msg)
		return perr
	})
	if err != nil {
		return -1, -1, err
	}
	return partition, offset, nil
}

func (g *guardedSyncProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	return g.exec.Execute(context.Background(), func(context.Context) error {
		return g.p.SendMessages(msgs)
	})
}

func (g *guardedSyncProducer) Close() error                            { return g.p.Close() }
func (g *guardedSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return g.p.TxnStatus() }
func (g *guardedSyncProducer) IsTransactional() bool                   { return g.p.IsTransactional() }
func (g *guardedSyncProducer) BeginTxn() error                         { return g.p.BeginTxn() }
func (g *guardedSyncProducer) CommitTxn() error                        { return g.p.CommitTxn() }
func (g *guardedSyncProducer) AbortTxn() error                         { return g.p.AbortTxn() }
func (g *guardedSyncProducer) AddOffsetsToTxn(o map[string][]*sarama.PartitionOffsetMetadata, gid string) error {
	return g.p.AddOffsetsToTxn(o, gid)
}
func (g *guardedSyncProducer) AddMessageToTxn(msg *sarama.ConsumerMessage, gid string, metadata *string) error {
	return g.p.AddMessageToTxn(msg, gid, metadata)
}
