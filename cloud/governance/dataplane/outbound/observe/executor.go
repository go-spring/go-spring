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

package observe

import (
	"context"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// WrapClientExecutor returns an [chain.Executor] that wraps inner with an internal span
// (system/service/status attributes), the resilience metrics, and an
// access log per call. Pass the system label (e.g. "redis", "gorm", "grpc")
// so calls from several protected clients are distinguishable. A nil inner
// returns nil — no wrapper, so an unarmed client stays untouched.
//
// This is an internal composition step, not a client API: the resilience
// manager applies it while the provider-built executor is still private, so
// clients get an already-observed executor and never wrap one themselves. The
// only other caller is a client that composes its own stack around a raw
// executor it built directly (e.g. httpx's custom chain.Executor / ResilienceDriver
// paths).
//
// The shared instruments are resolved at this point — the first resolve time,
// not package init — so an SDK installed after this package's init still
// receives the spans and records. The tracer is not held at all; see
// [observingExecutor.Execute].
func WrapClientExecutor(inner chain.Executor, system, service string) chain.Executor {
	if inner == nil {
		return nil
	}
	w := &observingExecutor{
		inner: inner,
		callEmitter: &callEmitter{
			system:  system,
			service: service,
			ins:     clientInstruments(),
		},
	}
	return w
}

type observingExecutor struct {
	inner chain.Executor
	*callEmitter
}

// Execute is the single emission point of a protected client call. It installs
// the per-call accumulator the retry loop inside the executor writes into, opens
// the call's span BEFORE handing the call inward — so every span the downstream
// opens (an otelhttp transport, a driver's own) nests under it — and, once the
// call returns, emits the call's signals from the two grains the accumulator
// separates: what each attempt cost the downstream, and what the whole call cost
// this caller.
//
// The slice of the chain is what makes this correct: the span is opened outside
// [defaultExecutor.Execute]'s retry loop, so it covers every attempt, and the
// emit runs after the loop, so it reads the attempts the loop recorded. The
// governance layer emits nothing itself; it only writes facts.
//
// A call that declared no operation (a zero [chain.Operation]) takes the
// fallback path below, which reproduces this layer's historical signals verbatim,
// down to their names — migrating a client to the declared-operation path is what
// switches its signals over, one client at a time.
func (w *observingExecutor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	return w.call(ctx, op, rec, func(c context.Context) error { return w.inner.Execute(c, op, rec, fn) }, false)
}

func (w *observingExecutor) Close() error { return w.inner.Close() }
