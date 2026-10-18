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

// command.go is the "produce/consume seam" concept of this starter: the
// InnerKafka chain the synchronous produces ride (see entity.go) and the
// per-record consume seam. franz-go's async Produce returns immediately, so
// only the synchronous ProduceSync path can be chained, and the poll loop is
// owned by whoever calls PollFetches, so consumption can only be guarded per
// record.
//
// The per-call signals are not emitted here: each entry point declares the
// operation's identity (see [operation]) on the ctx and hands the call to the
// resilience executor, which is the one place on the chain that sees the whole
// call and emits its span, metrics and access log.

package StarterKafka

import (
	"context"

	"go-spring.org/cloud/runtime/dataplane/kernel/label"
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"

	"github.com/twmb/franz-go/pkg/kgo"
	observability "go-spring.org/cloud/runtime/dataplane/outbound/observe"
)

// serviceLabelOf fixes the governance label a broker list scopes
// limiter/breaker state by.
func serviceLabelOf(brokers string) string {
	return label.ServiceLabel("kafka", brokers)
}

// runObserved runs call under the client's observer, which emits the produce's
// span, metrics and access log from the one point that sees the whole call. A
// kafka client has no seam for the resilience executor, so there is no
// governance on this path — observation only.
func runObserved(ctx context.Context, obs *observability.OperationObserver, op chain.Operation, call func(context.Context) error) error {
	return obs.Observe(ctx, op, nil, call)
}

// InnerKafka is the seam a client's synchronous produces run through.
// franz-go delivers a concrete client and its hooks only observe (they do not
// wrap or reject), so this interface is the ONLY way to modify what happens
// under the promoted ProduceSync.
//
// The default chain is the identity layer over the governance layer over a raw
// adapter, and the embedded InnerKafka is where a custom layer goes: implement
// this interface (embed the head you found to inherit the methods you do not
// care about), then assign your layer over it. The chain under the layer keeps
// doing its job — the topics a layer rewrites are what the identity layer
// declares, and the executor still protects every produce.
//
// franz-go exposes two produce APIs: Produce (async, callback on completion)
// and ProduceSync (blocks for broker ack). Only the synchronous path rides the
// chain; the async path returns immediately so an executor around it would be
// meaningless.
//
// Release follows the chain protocol; the raw client's own lifecycle (flush
// then close) belongs to [Client.Close], so the raw layer's Release is a
// pass-through at either depth.
type InnerKafka interface {
	// ProduceSync produces recs synchronously, blocking until the broker
	// acknowledges.
	ProduceSync(ctx context.Context, recs ...*kgo.Record) kgo.ProduceResults
	// Release releases the layer's own resources, then hands releaseRaw to
	// the layer under it.
	Release(releaseRaw bool) error
}

// RawKafka is the adapter layer: it makes the synchronous produce on the raw
// client. The W3C trace context rides the record headers via the kotel hooks
// the driver installs, so the adapter adds nothing of its own.
// [NewRawKafka] builds it.
type RawKafka struct{ cl *kgo.Client }

// NewRawKafka wraps a raw client as the chain's tail.
func NewRawKafka(cl *kgo.Client) *RawKafka { return &RawKafka{cl: cl} }

// Release is the protocol's pass-through: the client's flush-and-close belongs
// to [Client.Close], not the chain.
func (r *RawKafka) Release(bool) error { return nil }

func (r *RawKafka) ProduceSync(ctx context.Context, recs ...*kgo.Record) kgo.ProduceResults {
	return r.cl.ProduceSync(ctx, recs...)
}

// ProduceSync runs the produce under the observer and returns its results. A
// failure — a wire error returned by the tail — is captured so its status is
// classified; the results themselves are always returned, so a caller's
// .FirstErr() reads the same error the observer saw.
func (o *ObsKafka) ProduceSync(ctx context.Context, recs ...*kgo.Record) kgo.ProduceResults {
	var results kgo.ProduceResults
	_ = runObserved(ctx, o.obs, operation(opSend, topicOf(recs)), func(callCtx context.Context) error {
		results = o.next.ProduceSync(callCtx, recs...)
		return results.FirstErr()
	})
	return results
}

// ObsKafka is the emitting layer at the head: it names each produce — with the
// batch's topic (its first record's; an empty batch declares no destination) —
// runs it, and emits the span, metrics and access log through the client's
// observer. A kafka client has no seam for the resilience executor, so there is
// no governance layer under it. [NewObsKafka] builds it.
type ObsKafka struct {
	next InnerKafka
	obs  *observability.OperationObserver
}

// NewObsKafka builds the emitting layer over next.
func NewObsKafka(next InnerKafka, obs *observability.OperationObserver) *ObsKafka {
	return &ObsKafka{next: next, obs: obs}
}

// Release hands releaseRaw to the layer under it — this layer holds no
// resource.
func (o *ObsKafka) Release(releaseRaw bool) error { return o.next.Release(releaseRaw) }

// GuardedProduceSync produces recs synchronously on the client, routed through
// its chain. When governance is disabled this behaves exactly like
// Kafka.ProduceSync. On rejection (rate-limit or open circuit) the returned
// ProduceResults carries the rejection error on every record, so .FirstErr()
// surfaces the sentinel just like a real produce failure and the underlying
// produce is never invoked. It is a thin convenience over the embedded
// [InnerKafka] head.
func GuardedProduceSync(ctx context.Context, cl *Client, recs ...*kgo.Record) kgo.ProduceResults {
	return cl.InnerKafka.ProduceSync(ctx, recs...)
}

// GuardedConsume runs one consumed record's handler under the client's
// observer, naming the consume from the record's topic first. It is the consume
// counterpart of [GuardedProduceSync], for an application that owns its own poll
// loop.
//
// The managed subscription ([NewDriver]'s subscriber) runs through this same
// function, so the two paths cannot drift: a record handled here carries the
// same span name, the same messaging.* metrics and the same access log as one
// delivered by the driver.
//
// It is not a chain method because its pipeline is per-record and driven by
// whoever owns the poll loop — franz-go's hooks observe a record, they do not
// wrap the call — so it uses the client's observer seam instead.
func GuardedConsume(ctx context.Context, cl *Client, rec *kgo.Record, handler func(context.Context) error) error {
	return cl.observe(ctx, operation(opProcess, rec.Topic), handler)
}

// topicOf returns the destination of a produce batch: its topic when EVERY
// record in the batch shares one, and "" otherwise.
//
// A batch is one operation with one destination or with none. Taking the first
// record's topic would attribute a mixed batch to whichever topic happened to be
// first — a claim that reaches the span NAME and the governance rule key, not
// just a detail field, so a rule aimed at one topic would match a batch that
// only partially went there. An empty batch and a mixed one both declare no
// destination; a success line then stays at Info rather than naming a topic that
// was never the whole story.
func topicOf(recs []*kgo.Record) string {
	if len(recs) == 0 {
		return ""
	}
	topic := recs[0].Topic
	for _, r := range recs[1:] {
		if r.Topic != topic {
			return ""
		}
	}
	return topic
}
