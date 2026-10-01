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
// resilience layer (the [AttachGovernance] attach plus the clientGuards
// registry) and the two guarded entry points an application calls instead of the
// client's own methods — [GuardedProduceSync] and [GuardedConsume]. They are the
// only places the resilience seam can attach: franz-go's async Produce returns
// immediately, so only the synchronous ProduceSync path can be wrapped, and its
// poll loop is owned by whoever calls PollFetches, so consumption can only be
// guarded per record.
//
// The per-call signals are not emitted here: each entry point declares the
// operation's identity (see [operation]) on the ctx and hands the call to the
// resilience executor, which is the one place on the chain that sees the whole
// call and emits its span, metrics and access log.
package StarterKafka

import (
	"context"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/cloud"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
)

// clientGuard is the per-client resilience attachment: the executor chain and
// the stable serviceLabel it executes under, colocated so a guard lookup
// reads the pair atomically (no torn exec/serviceLabel combination).
type clientGuard struct {
	exec         resilience.ClientExecutor
	serviceLabel string
}

// clientGuards indexes the guard by the raw client bean, so the package-level
// GuardedProduceSync can resolve it from a bare *kgo.Client and the destructor
// can Close it. Only clients with resilience enabled appear here.
var clientGuards sync.Map // *kgo.Client -> *clientGuard

// AttachGovernance completes a franz-go client with the container's
// service-governance capabilities: it resolves the client's executor from params
// and indexes it by the raw client, so [GuardedProduceSync] and the consumer
// loop can find it from the bare *kgo.Client. serviceLabel scopes the
// limiter/breaker state per instance.
//
// This is the kafka (franz-go) seam of resilience: franz-go's async Produce
// returns immediately (a record is handed to the internal producer and a
// callback fires on completion), so wrapping it in exec.Execute has no meaning.
// The synchronous ProduceSync path, which blocks until the broker acknowledges,
// is what GuardedProduceSync protects.
//
// It is called by [DefaultDriver.CreateClient] while the client is being built —
// governance is applied IN the constructor, not by a later step — and a custom
// [Driver] calls it for the same reason: the client is only in the driver's
// hands, so this is the one place the guard can be attached. brokers is the
// client's identity; the executor is the one [cloud.ClientParams.ExecutorFor]
// composes — the resilience authority's executor wrapped with fault injection
// when the bundle is populated, and the observe-only [resilience.Unmanaged] one
// when it is zero, so a client assembled without a container is still observed
// (and says so once) rather than running bare.
func AttachGovernance(cl *kgo.Client, brokers string, params cloud.ClientParams) {
	serviceLabel := resilience.ServiceLabel("kafka", brokers)
	clientGuards.Store(cl, &clientGuard{
		exec:         params.ExecutorFor("kafka", serviceLabel),
		serviceLabel: serviceLabel,
	})
}

// closeResilience closes and forgets the guard behind cl, if any.
func closeResilience(cl *kgo.Client) {
	if v, ok := clientGuards.LoadAndDelete(cl); ok {
		if err := v.(*clientGuard).exec.Close(); err != nil {
			log.Warnf(context.Background(), log.TagAppDef, "kafka: resilience executor close failed: %v", err)
		}
	}
}

// guardOf resolves the resilience executor attached to cl, or nil when the
// client carries none (a stand-alone client, e.g. built outside the starter).
func guardOf(cl *kgo.Client) resilience.ClientExecutor {
	if v, ok := clientGuards.Load(cl); ok {
		return v.(*clientGuard).exec
	}
	return nil
}

// guard routes call through the executor attached to cl, and otherwise runs it
// inline. When resilience is disabled for the client this is a no-op
// pass-through, so enabling protection is a zero-code opt-in on the caller side.
//
// Routing goes through [resilience.Run] — the shared client-operation body —
// rather than the executor's Execute directly, so the nil-executor pass-through
// and the fault-injection edge (an attempt short-circuited before call ran) are
// classified in one place. The call's declared identity (see [operation]) rides
// the ctx and is read by the executor, which emits the span, the metrics and the
// access log.
func guard(ctx context.Context, cl *kgo.Client, call func(context.Context) error) error {
	_, err := resilience.Run(ctx, guardOf(cl),
		func(attemptCtx context.Context) (struct{}, error) {
			return struct{}{}, call(attemptCtx)
		})
	return err
}

// GuardedProduceSync produces recs synchronously on cl, routed through the
// resilience executor attached to cl when governance is enabled. When
// governance is disabled this behaves exactly like cl.ProduceSync. On
// rejection (rate-limit or open circuit) the returned ProduceResults carries
// the rejection error on every record, so .FirstErr() surfaces the sentinel
// just like a real produce failure and the underlying produce is never invoked.
//
// The publish's identity is declared here, from the first record's topic, before
// the call enters the executor — so the executor's span, metrics and access log
// are named `publish` and carry the topic (see [operation]).
//
// franz-go exposes two produce APIs: Produce (async, callback on completion)
// and ProduceSync (blocks for broker ack). Only the synchronous path is
// guarded here; the async path returns immediately so an executor around
// it would be meaningless.
func GuardedProduceSync(ctx context.Context, cl *kgo.Client, recs ...*kgo.Record) kgo.ProduceResults {
	ctx = observability.WithOperation(ctx, operation(opPublish, topicOf(recs)))
	var results kgo.ProduceResults
	err := guard(ctx, cl, func(attemptCtx context.Context) error {
		results = cl.ProduceSync(attemptCtx, recs...)
		return results.FirstErr()
	})
	if err != nil {
		// Rejected before the produce ran: encode the rejection as a per-record
		// error so the caller's .FirstErr() surfaces it transparently.
		out := make(kgo.ProduceResults, 0, len(recs))
		for _, r := range recs {
			out = append(out, kgo.ProduceResult{Record: r, Err: err})
		}
		return out
	}
	return results
}

// GuardedConsume runs one consumed record's handler under the resilience
// executor attached to cl, declaring the consume's identity from the record's
// topic first. It is the consume counterpart of [GuardedProduceSync], for an
// application that owns its own poll loop.
//
// The managed subscription ([NewDriver]'s subscriber) runs through this same
// function, so the two paths cannot drift: a record handled here carries the
// same span name, the same messaging.* metrics and the same access log as one
// delivered by the driver.
//
// It exists because the raw *kgo.Client bean cannot be made to route its own
// PollFetches through the guard — franz-go's hooks observe a record, they do not
// wrap the call — so an application that polls directly would otherwise consume
// with no limiter/breaker/retry/timeout, no messaging.* metrics and no access
// log, leaving only kotel's client-level telemetry. Calling this once per record
// restores the governed path without giving up the raw loop.
//
// Like [GuardedProduceSync], the declaration is made before the executor runs:
// the emitter reads the operation at Execute entry, so one declared per attempt
// would be read by nobody.
func GuardedConsume(ctx context.Context, cl *kgo.Client, rec *kgo.Record, handler func(context.Context) error) error {
	ctx = observability.WithOperation(ctx, operation(opConsume, rec.Topic))
	return guard(ctx, cl, handler)
}

// topicOf returns the destination of a produce batch, taken from its first
// record — a batch produced through one call targets one topic, and an empty
// batch declares no destination (no Detail, so a success stays at Info).
func topicOf(recs []*kgo.Record) string {
	if len(recs) == 0 {
		return ""
	}
	return recs[0].Topic
}
