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

	"go-spring.org/cloud/governance/resilience"
)

// WrapExecutor returns an Executor that injects faults into fn before
// delegating to inner.
//
// The injection happens INSIDE inner's retry loop: the wrapped fn either
// returns the injected error (so the real executor retries, the breaker counts
// the failure, and observe records the outcome) or calls the real fn. This is
// what makes "setting fire" validate the resilience stack — a fault flows
// through retry/breaker/timeout/Fallback exactly as a real downstream failure
// would, rather than short-circuiting at the boundary where none of those
// mechanisms are in play.
//
// The process-wide injector ([InjectorFor]) is resolved LAZILY on each Execute:
// this lets client starters wrap at wiring time (in their InitMethod) even
// though starter-govern registers the injector later, at Runner time — mirroring
// how [resilience.ExecutorFor] defers provider resolution to call time. When no
// injector is registered, fn runs once untouched (zero-config transparency).
//
// nil inner => returns nil.
func WrapExecutor(inner resilience.Executor) resilience.Executor {
	if inner == nil {
		return inner
	}
	return &faultExecutor{inner: inner}
}

// WrapExecutorWith is [WrapExecutor] with an explicitly supplied injector: in
// is used directly on every Execute instead of resolving [InjectorFor]. This is
// the path tests and [cloud/loadtest] take to inject an explicit, locally-built
// injector.
//
// nil inner => returns nil.
func WrapExecutorWith(inner resilience.Executor, in *Injector) resilience.Executor {
	if inner == nil {
		return inner
	}
	return &faultExecutor{inner: inner, in: in}
}

type faultExecutor struct {
	inner resilience.Executor
	in    *Injector
}

// Execute wraps fn so each attempt is faulted per the injector's live config
// before the real executor sees it. An injected latency sleeps first (cancellable
// via the attempt context); if the sleep is cancelled the context error is
// returned so the executor's budget/timeout logic reacts rather than retrying
// blindly.
//
// When e.in is nil (the lazy-global path) the injector is resolved here, per
// Execute, via [InjectorFor]; nil means no fault is configured and inner runs fn
// untouched.
func (e *faultExecutor) Execute(ctx context.Context, resource string, fn func(context.Context) error) error {
	in := e.in
	if in == nil {
		in = InjectorFor()
	}
	if in == nil {
		return e.inner.Execute(ctx, resource, fn)
	}
	// Each attempt runs through the injector's shared Gate sequence (scope
	// gating, guardrails, latency, error) before the real fn — a fault flows
	// through retry/breaker/timeout exactly as a real downstream failure would.
	wrapped := func(attemptCtx context.Context) error {
		return in.Gate(attemptCtx, resource, fn)
	}
	return e.inner.Execute(ctx, resource, wrapped)
}

// Close releases the inner executor's resources.
func (e *faultExecutor) Close() error { return e.inner.Close() }

// Refresh forwards the new policy to the inner executor. The fault injector
// itself has no policy to refresh — its own config is swapped via
// [Injector.SetConfig] from the starter's gs.Dync binding.
func (e *faultExecutor) Refresh(p resilience.Policy) error { return e.inner.Refresh(p) }
