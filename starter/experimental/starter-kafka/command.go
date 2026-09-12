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

// command.go is the "produce seam" concept of this starter: the observe layer
// (newObserveHook/observeHook, the per-message access log) and the resilience
// layer (guard/GuardedProduceSync/applyResilience plus the resilienceExecs and
// resilienceResources registries). franz-go's async Produce returns immediately,
// so only the synchronous ProduceSync path is guarded; GuardedProduceSync is the
// single produce entry point the resilience seam protects.
package StarterKafka

import (
	"context"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
)

// newObserveHook builds a kgo hook that emits a per-message access log (see
// observe.go). kotel already provides producer/consumer spans and client
// metrics, so this hook is log-only: it fills the access-log gap without
// duplicating spans or metrics. The produce path pairs
// OnProduceRecordBuffered (start) with OnProduceRecordUnbuffered (end) so the
// log carries accurate duration; consume has no paired start hook, so it emits
// a duration-less record (the metric covers consume latency).
func newObserveHook() kgo.Hook {
	return &observeHook{}
}

type observeHook struct {
	spans sync.Map // *kgo.Record -> *accessRecord (in-flight produces)
}

// OnProduceRecordBuffered opens an access record when a record is queued.
func (h *observeHook) OnProduceRecordBuffered(r *kgo.Record) {
	h.spans.Store(r, startAccess(context.Background(), "publish", r.Topic))
}

// OnProduceRecordUnbuffered closes the access record when the record is
// acknowledged (or fails), recording the outcome and the buffered→unbuffered
// duration.
func (h *observeHook) OnProduceRecordUnbuffered(r *kgo.Record, err error) {
	if v, ok := h.spans.LoadAndDelete(r); ok {
		v.(*accessRecord).End(err)
	}
}

// OnFetchRecordRead emits a consume access record per message read.
func (h *observeHook) OnFetchRecordRead(r *kgo.Record) {
	startAccess(context.Background(), "consume", r.Topic).End(nil)
}

// clientGuard is the per-client resilience attachment: the executor chain and
// the stable resource label it executes under, colocated so a guard lookup
// reads the pair atomically (no torn exec/resource combination).
type clientGuard struct {
	exec     resilience.Executor
	resource string
}

// clientGuards indexes the guard by the raw client bean, so the package-level
// GuardedProduceSync can resolve it from a bare *kgo.Client and the destructor
// can Close it. Only clients with resilience enabled appear here.
var clientGuards sync.Map // *kgo.Client -> *clientGuard

// applyResilience builds an executor and indexes it by cl. This is the kafka
// (franz-go) seam of resilience: franz-go's async Produce returns immediately
// (a record is handed to the internal producer and a callback fires on
// completion), so wrapping it in exec.Execute has no meaning. The synchronous
// ProduceSync path, which blocks until the broker acknowledges, is what
// GuardedProduceSync protects. resource scopes the limiter/breaker state.
//
// Both the executor and the fault injector are resolved through neutral seams
// ([resilience.ExecutorFor] / [fault.InjectorFor]) that starter-govern backs with
// the governance center — so this function has zero coupling to cloud/governance.
// When governance is off, ExecutorFor yields a transparent no-op executor; fault
// wraps it when an injector is registered (nil-safe otherwise).
func applyResilience(cl *kgo.Client, resource string) error {
	exec := fault.WrapExecutor(resilience.ExecutorFor("kafka", resource))
	clientGuards.Store(cl, &clientGuard{exec: exec, resource: resource})
	return nil
}

// closeResilience closes and forgets the guard behind cl, if any.
func closeResilience(cl *kgo.Client) {
	if v, ok := clientGuards.LoadAndDelete(cl); ok {
		if err := v.(*clientGuard).exec.Close(); err != nil {
			log.Warnf(context.Background(), log.TagAppDef, "kafka: resilience executor close failed: %v", err)
		}
	}
}

// guard routes call through the executor attached to cl, and otherwise runs it
// inline. When resilience is disabled for the client this is a no-op
// pass-through, so enabling protection is a zero-code opt-in on the caller side.
func guard(ctx context.Context, cl *kgo.Client, call func(context.Context) error) error {
	v, ok := clientGuards.Load(cl)
	if !ok {
		return call(ctx)
	}
	g := v.(*clientGuard)
	return g.exec.Execute(ctx, g.resource, call)
}

// GuardedProduceSync produces recs synchronously on cl, routed through the
// resilience executor attached to cl when governance is enabled. When
// governance is disabled this behaves exactly like cl.ProduceSync. On
// rejection (rate-limit or open circuit) the returned ProduceResults carries
// the rejection error on every record, so .FirstErr() surfaces the sentinel
// just like a real produce failure and the underlying produce is never invoked.
//
// franz-go exposes two produce APIs: Produce (async, callback on completion)
// and ProduceSync (blocks for broker ack). Only the synchronous path is
// guarded here; the async path returns immediately so an exec.Execute around
// it would be meaningless.
func GuardedProduceSync(ctx context.Context, cl *kgo.Client, recs ...*kgo.Record) kgo.ProduceResults {
	var results kgo.ProduceResults
	err := guard(ctx, cl, func(ctx context.Context) error {
		results = cl.ProduceSync(ctx, recs...)
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
