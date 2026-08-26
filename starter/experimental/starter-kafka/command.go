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
	observe "go-spring.org/cloud/observe"
	resilobserve "go-spring.org/cloud/observe/resilience"
)

// newObserveHook builds a kgo hook that emits a per-message access log through
// the observe kit. kotel already provides producer/consumer spans and client
// metrics, so this hook is log-only (WithoutTraceAndMetric): it fills the
// access-log gap without duplicating spans or metrics. The produce path pairs
// OnProduceRecordBuffered (start) with OnProduceRecordUnbuffered (end) so the
// log carries accurate duration; consume has no paired start hook, so it emits
// a duration-less record (the metric covers consume latency).
func newObserveHook(cfg observe.ObserveConfig) kgo.Hook {
	return &observeHook{
		pubObs: observe.NewProducer("kafka", cfg, observe.WithoutTraceAndMetric()),
		subObs: observe.NewConsumer("kafka", cfg, observe.WithoutTraceAndMetric()),
	}
}

type observeHook struct {
	pubObs *observe.Observer
	subObs *observe.Observer
	spans  sync.Map // *kgo.Record -> *observe.Span (in-flight produces)
}

// OnProduceRecordBuffered opens a producer observation when a record is queued.
func (h *observeHook) OnProduceRecordBuffered(r *kgo.Record) {
	_, sp := h.pubObs.Start(context.Background(), "publish", r.Topic)
	h.spans.Store(r, sp)
}

// OnProduceRecordUnbuffered closes the producer observation when the record is
// acknowledged (or fails), recording the outcome and the buffered→unbuffered
// duration.
func (h *observeHook) OnProduceRecordUnbuffered(r *kgo.Record, err error) {
	if v, ok := h.spans.LoadAndDelete(r); ok {
		v.(*observe.Span).End(err)
	}
}

// OnFetchRecordRead emits a consume access record per message read.
func (h *observeHook) OnFetchRecordRead(r *kgo.Record) {
	_, sp := h.subObs.Start(context.Background(), "consume", r.Topic)
	sp.End(nil)
}

// resilienceExecs tracks the resilience executor attached to each client, so
// GuardedProduceSync can resolve it from the raw *kgo.Client bean and the
// destructor can Close it (releasing any background resources of a production
// driver). Only clients with resilience enabled appear here.
var resilienceExecs sync.Map // *kgo.Client -> resilience.Executor

// resilienceResources tracks the stable resource label per client so the guard
// can pass it to exec.Execute without re-deriving from Config.
var resilienceResources sync.Map // *kgo.Client -> string

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
func applyResilience(c Config, cl *kgo.Client, resource string) error {
	exec := fault.WrapExecutor(resilience.ExecutorFor(resource))
	exec = resilobserve.WrapExecutor(exec, "kafka", c.Observability)
	resilienceExecs.Store(cl, exec)
	resilienceResources.Store(cl, resource)
	return nil
}

// closeResilience closes and forgets the executor behind cl, if any.
func closeResilience(cl *kgo.Client) {
	if v, ok := resilienceExecs.LoadAndDelete(cl); ok {
		_ = v.(resilience.Executor).Close()
	}
	resilienceResources.Delete(cl)
}

// guard routes call through the executor attached to cl, and otherwise runs it
// inline. When resilience is disabled for the client this is a no-op
// pass-through, so enabling protection is a zero-code opt-in on the caller side.
func guard(ctx context.Context, cl *kgo.Client, call func(context.Context) error) error {
	v, ok := resilienceExecs.Load(cl)
	if !ok {
		return call(ctx)
	}
	r, _ := resilienceResources.Load(cl)
	return v.(resilience.Executor).Execute(ctx, r.(string), call)
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
