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

// command.go is the "command seam" concept of this starter: the publish and
// consume seams, each stacking a declaration layer over the resilience layer.
//
//	declare    — WrapSyncProducer / Consume build the operation's identity
//	             (see [operation]) from the direction and topic they alone know,
//	             put it on the ctx with [observability.WithOperation], and
//	             inject/extract the W3C trace context (and the load-test marker)
//	             across the broker.
//	resilience — AttachGovernance / guardedSyncProducer / executorFor, indexed per
//	             client by sync.Map. The executor is the single emitter: it reads
//	             the declared operation off the ctx and opens the span, records
//	             the durations (call-level messaging.client.operation.duration and
//	             attempt-level messaging.client.attempt.duration) and writes the
//	             one access log.
//
// Neither seam emits anything itself. Declaring the identity is this layer's
// whole job now.
//
// Why the call-site helpers rather than a wrapped producer/consumer:
//
//  1. The only official OTel instrumentation for sarama, otelsarama
//     (go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/
//     otelsarama), is deprecated and still pinned to the abandoned
//     github.com/Shopify/sarama module. This starter uses github.com/IBM/sarama;
//     the two are distinct Go types, so otelsarama's WrapSyncProducer cannot wrap
//     an IBM producer, and importing it would drag in a second, conflicting
//     sarama fork. We therefore declare natively on the ctx and let the
//     resilience layer emit.
//
//  2. sarama.SyncProducer.SendMessage takes no context.Context, so a producer
//     wrapper has nowhere to receive the caller's context from. The declaration
//     therefore has to be built inside the wrapper, from the message the call
//     carries; the publish and consume seams take the message and derive the
//     operation from it.
//
// Everything here rides the resilience executor, which in turn rides the OTel
// globals that starter-otel installs. Without starter-otel the global providers
// are no-ops, so the declaration and the wrap cost almost nothing and change no
// message bytes.
package StarterKafkaSarama

import (
	"context"
	"go-spring.org/cloud/chain"
	"sync"

	"github.com/IBM/sarama"
	"go-spring.org/cloud"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/propagate"
	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/traffic"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// propagatorOr returns prop, or the default load-test convention when prop is
// nil. DefaultBinding is complete, so building the default cannot fail.
func propagatorOr(prop traffic.Propagator) traffic.Propagator {
	if prop == nil {
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return prop
}

// injectProducer stamps the current trace context (W3C) and the load-test
// marker into msg's headers, so a downstream consumer can continue the trace
// and recognise synthetic load. It is called from inside the attempt, so the
// traceparent carries the executor's span. producerCarrier's Set is idempotent,
// so re-injection on a retry never appends a second header.
func injectProducer(ctx context.Context, msg *sarama.ProducerMessage, prop traffic.Propagator) {
	otel.GetTextMapPropagator().Inject(ctx, producerCarrier{msg})
	prop.Inject(ctx, producerCarrier{msg})
}

// extractConsumer pulls the upstream trace context (W3C) and the load-test
// marker out of msg's headers into ctx. Extraction never mutates the message.
func extractConsumer(ctx context.Context, msg *sarama.ConsumerMessage, prop traffic.Propagator) context.Context {
	ctx = otel.GetTextMapPropagator().Extract(ctx, consumerCarrier{msg})
	return prop.Extract(ctx, consumerCarrier{msg})
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

// guard routes call through the executor when one is attached, and otherwise
// runs it inline. Splitting this out keeps the guarded methods trivial and
// makes the pass-through / rejection paths independently testable without a
// live broker.
func guard(exec chain.Executor, ctx context.Context, call func(context.Context) error) error {
	if exec == nil {
		return call(ctx)
	}
	return exec.Execute(ctx, call)
}

// clientGuard is the per-client resilience attachment: the executor chain and
// the stable serviceLabel it executes under, colocated so a guard lookup
// reads the pair atomically (no torn exec/serviceLabel combination).
type clientGuard struct {
	exec         chain.Executor
	serviceLabel string
}

// clientGuards indexes the guard by the raw client bean, so WrapSyncProducer and
// Consume can resolve it from a bare sarama.Client and the destructor can Close
// it. Only clients with resilience enabled appear here.
var clientGuards sync.Map // sarama.Client -> *clientGuard

// AttachGovernance builds a client's resilience executor from the governance
// bundle the driver was handed and indexes it by client. It is called by the
// driver the moment the client is built (driver.go), so a client is never
// observed half-assembled and there is no post-hoc step for the container to
// remember to run — the client is complete when the constructor returns.
//
// This is the kafka-sarama seam of resilience: sarama exposes no reject-capable
// middleware, so the executor is driven through the call-site seams (see
// WrapSyncProducer and Consume) that callers opt into once per derived producer /
// consumption site.
//
// params carries the container's service-governance capabilities: the governed
// executor (fault-wrapped) when the container is present, and the observed-only
// [resilience.Unmanaged] one — with a one-time warning — when it is not, so a
// client assembled by hand (or by an example) is never silently unprotected.
func AttachGovernance(client sarama.Client, brokers string, params cloud.ClientParams) {
	serviceLabel := resilience.ServiceLabel(kafkaSystem, brokers)
	clientGuards.Store(client, &clientGuard{
		exec:         params.ExecutorFor(kafkaSystem, serviceLabel),
		serviceLabel: serviceLabel,
	})
}

// closeResilience closes and forgets the executor behind client, if any.
func closeResilience(client sarama.Client) {
	if v, ok := clientGuards.LoadAndDelete(client); ok {
		_ = v.(*clientGuard).exec.Close()
	}
}

// executorFor loads the executor and serviceLabel attached to client. Returns
// (nil, "") when resilience is disabled for that client, so the seams fall
// back to a direct call.
func executorFor(client sarama.Client) (chain.Executor, string) {
	v, ok := clientGuards.Load(client)
	if !ok {
		return nil, ""
	}
	g := v.(*clientGuard)
	return g.exec, g.serviceLabel
}

// WrapSyncProducer returns a sarama.SyncProducer that declares the publish's
// identity and routes SendMessage / SendMessages through the resilience executor
// attached to cl when governance is enabled. The executor is the single emitter:
// it opens the span (named "publish", with the topic as messaging.destination.name
// on the span and in the log), records messaging.client.operation.duration and
// messaging.client.attempt.duration, and writes the one access log — declaring
// the identity is all this wrapper does.
//
// The wrapper is ALWAYS applied (also when governance is off) because it is also
// where the W3C trace context and the load-test marker are injected into the
// message headers — propagation is not emission and must happen either way. When
// no executor is attached the declaration and the send still run, just inline
// with no span. Callers therefore always wrap unconditionally:
//
//	prod, _ := sarama.NewSyncProducerFromClient(cl)
//	prod = StarterKafkaSarama.WrapSyncProducer(cl, prod, prop)
//	_, _, err := prod.SendMessage(msg) // now declared, traced / rate-limited / circuit-guarded
//
// prop is the process's load-test convention; a nil propagator falls back to
// [traffic.NewDefaultPropagator] with [traffic.DefaultBinding].
//
// Only the synchronous send paths are guarded; the transaction and Close methods
// delegate directly, as does TxnStatus/IsTransactional. SendMessage takes no
// context.Context (sarama's API is context-free), so the call is declared from
// the message alone and starts from context.Background() — the call's span is
// therefore a NEW ROOT, and a per-call deadline is expressed via
// AttemptTimeout/MaxDuration in [resilience.Config] when a bound matters.
func WrapSyncProducer(cl sarama.Client, p sarama.SyncProducer, prop traffic.Propagator) sarama.SyncProducer {
	if p == nil {
		return p
	}
	exec, serviceLabel := executorFor(cl)
	return &guardedSyncProducer{p: p, exec: exec, serviceLabel: serviceLabel, prop: propagatorOr(prop)}
}

// guardedSyncProducer is a transparent sarama.SyncProducer wrapper that declares
// each send's identity and drives the synchronous send methods through the
// resilience executor. All other methods (Close, transaction lifecycle, status
// queries) delegate to the inner producer unchanged — they are control-plane,
// not the protected data path.
type guardedSyncProducer struct {
	p            sarama.SyncProducer
	exec         chain.Executor
	serviceLabel string
	prop         traffic.Propagator
}

var _ sarama.SyncProducer = (*guardedSyncProducer)(nil)

func (g *guardedSyncProducer) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	ctx := observability.WithOperation(context.Background(), operation(opPublish, msg.Topic))
	err = guard(g.exec, ctx, func(attemptCtx context.Context) error {
		injectProducer(attemptCtx, msg, g.prop)
		var perr error
		partition, offset, perr = g.p.SendMessage(msg)
		return perr
	})
	if err != nil {
		return -1, -1, err
	}
	return partition, offset, nil
}

// SendMessages declares one publish for the batch (addressed by the first
// message's topic) and routes it through the executor, injecting the trace
// context into every message. sarama's batch call is a single call from this
// caller's point of view, so it is one operation.
func (g *guardedSyncProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	topic := ""
	if len(msgs) > 0 {
		topic = msgs[0].Topic
	}
	ctx := observability.WithOperation(context.Background(), operation(opPublish, topic))
	return guard(g.exec, ctx, func(attemptCtx context.Context) error {
		for _, m := range msgs {
			injectProducer(attemptCtx, m, g.prop)
		}
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

// Consume runs handle for one consumed message under the resilience executor
// attached to cl: it extracts the upstream W3C trace context and the load-test
// marker from msg into ctx, declares the consume's identity, and runs handle
// under the executor — which emits the span (named "consume", the topic as
// messaging.destination.name), the messaging.client.operation.duration /
// messaging.client.attempt.duration metrics and the one access log. Offset
// commits are entirely the caller's / consumer group's concern; this never
// touches them.
//
// sarama hands out messages through a channel, so there is no wrapper to own the
// consumption; this is the call-site seam, invoked once per received message:
//
//	select {
//	case msg := <-pc.Messages():
//	    return StarterKafkaSarama.Consume(ctx, cl, msg, prop, func(ctx context.Context) error {
//	        return handle(ctx, msg)
//	    })
//	}
//
// The consume ctx's parent is taken from the message headers, so the consume
// span is a child of the producer's — provided both sides use these seams. With
// no executor attached (a standalone, non-gs client) handle still runs, inline,
// with the extracted context. prop is the process's load-test convention; a nil
// propagator falls back to the default.
func Consume(ctx context.Context, cl sarama.Client, msg *sarama.ConsumerMessage, prop traffic.Propagator, handle func(context.Context) error) error {
	ctx = extractConsumer(ctx, msg, propagatorOr(prop))
	ctx = observability.WithOperation(ctx, operation(opConsume, msg.Topic))
	exec, _ := executorFor(cl)
	return guard(exec, ctx, handle)
}
