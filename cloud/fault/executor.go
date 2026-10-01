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

package fault

import (
	"context"

	"go-spring.org/cloud/resilience"
)

// WrapClientExecutor returns an Executor that injects faults into fn before
// delegating to inner, using in as the injector.
//
// The injection happens INSIDE inner's retry loop: the wrapped fn either
// returns the injected error (so the real executor retries, the breaker counts
// the failure, and observe records the outcome) or calls the real fn. This is
// what makes "setting fire" validate the resilience stack — a fault flows
// through retry/breaker/timeout/Fallback exactly as a real downstream failure
// would, rather than short-circuiting at the boundary where none of those
// mechanisms are in play.
//
// in is the injector bean the caller received from the container; holding the
// pointer is enough to observe config changes, because the injector swaps its
// config in place ([Injector.SetConfig]) rather than being replaced. A nil in —
// the caller injected nothing, i.e. the governance starter is absent — returns
// inner unwrapped: with no governance there is no fault layer, so the wrap is
// always safe to apply.
//
// service is the label this executor protects, and the scope the injector's own
// rules are matched against; it is the same label inner was built for, so a
// fault rule and a resilience rule name the same service. The injector's CLIENT
// side supplies the faults here (see [Injector.gate]); the server side is
// [ApplyServer]'s and never touches an outbound call.
//
// nil inner => returns nil.
func WrapClientExecutor(inner resilience.ClientExecutor, service string, in *Injector) resilience.ClientExecutor {
	if inner == nil {
		return nil
	}
	if in == nil {
		return inner
	}
	return &faultExecutor{inner: inner, service: service, in: in}
}

type faultExecutor struct {
	inner   resilience.ClientExecutor
	service string
	in      *Injector
}

// Execute wraps fn so each attempt is faulted per the injector's live config
// before the real executor sees it. An injected latency sleeps first (cancellable
// via the attempt context); if the sleep is cancelled the context error is
// returned so the executor's budget/timeout logic reacts rather than retrying
// blindly.
func (e *faultExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Each attempt runs through the injector's shared injection sequence (scope
	// gating, guardrails, latency, error) on the CLIENT side before the real fn —
	// a fault flows through retry/breaker/timeout exactly as a real downstream
	// failure would.
	return e.inner.Execute(ctx, func(attemptCtx context.Context) error {
		return e.in.gate(attemptCtx, &e.in.client, e.service, fn)
	})
}

// Close releases the inner executor's resources.
func (e *faultExecutor) Close() error { return e.inner.Close() }

// Refresh forwards the new policy to the inner executor. The fault injector
// itself has no policy to refresh — its own config is swapped via
// [Injector.SetConfig] whenever the governance source pushes a new config.
func (e *faultExecutor) Refresh(p resilience.ClientPolicy) error { return e.inner.Refresh(p) }
