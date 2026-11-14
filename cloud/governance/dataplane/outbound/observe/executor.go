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

// WrapClientExecutor returns an [chain.Executor] that runs next under an
// internal span (system/service/status attributes), the resilience metrics, and
// an access log per call. Pass the system label (e.g. "redis", "gorm", "grpc")
// so calls from several protected clients are distinguishable. next nil means
// this layer is the terminal: the call itself is fn (see [chain.Next]).
//
// This is an internal composition step, not a client API: a composition root
// layers it over the rest of the chain, so clients get an already-observed
// executor and never wrap one themselves.
//
// The shared instruments are resolved at this point — the first resolve time,
// not package init — so an SDK installed after this package's init still
// receives the spans and records. The tracer is not held at all; see
// [observingExecutor.Execute].
func WrapClientExecutor(next chain.Executor, system, service string, m Metrics) chain.Executor {
	return &observingExecutor{
		next: next,
		callEmitter: &callEmitter{
			system:  system,
			service: service,
			metrics: m,
		},
	}
}

type observingExecutor struct {
	next chain.Executor
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
// The operation is the call's DECLARATION — every adapter builds one, including
// for a call it only wants quieted — so this layer emits exactly the signals the
// declaration describes and has no second vocabulary to fall back to.
func (w *observingExecutor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn chain.RoundFunc) error {
	return w.call(ctx, op, rec, func(c context.Context) error {
		return chain.Next(w.next, c, op, rec, fn)
	}, w.next == nil)
}

func (w *observingExecutor) Close() error {
	if w.next == nil {
		return nil
	}
	return w.next.Close()
}
