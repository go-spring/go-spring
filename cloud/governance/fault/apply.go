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
	"go-spring.org/cloud/governance/traffic"
)

// scopeApplies reports whether fault injection should run for a call, given the
// live config's Scope and the call's load-test marker. Scope "" (or unknown)
// injects into all traffic; "real" skips load-test calls; "loadtest" skips real
// calls. It is the shared gate used by both the client-side [WrapExecutor] and
// the server-side [Apply], so the two directions honour the same scoping rule.
func scopeApplies(c Config, ctx context.Context) bool {
	switch c.Scope {
	case "real":
		return !traffic.IsLoadTest(ctx)
	case "loadtest":
		return traffic.IsLoadTest(ctx)
	default:
		return true
	}
}

// Gate gates fn with the injector's fault rules, applying the configured
// latency before fn and returning the injected error instead of calling fn
// when the rate hits. It is the ONE shared fault sequence — both directions
// (the client-side [WrapExecutor] per-attempt closure and the server-side
// [Apply]) run through it, so scope gating, guardrails and cancellation
// semantics cannot drift apart.
//
// fn receives the caller's (or the executor's per-attempt) context so
// cancellation propagates into the real operation. The latency sleep is
// cancellable via ctx; on cancel the context error is returned so the
// caller's own timeout/budget logic reacts rather than retrying blindly.
// Gate does NOT retry — retry is a client concern; for the full
// retry/timeout/breaker treatment on a client call use [WrapExecutor] with a
// resilience.Executor.
func (in *Injector) Gate(ctx context.Context, resource string, fn func(context.Context) error) error {
	c := in.Config()
	if !c.Enabled || !scopeApplies(c, ctx) {
		return fn(ctx)
	}
	inject, sleep, injErr := in.maybe(resource)
	if sleep > 0 && !resilience.SleepFor(ctx, sleep) {
		return ctx.Err()
	}
	if inject {
		return injErr
	}
	return fn(ctx)
}

// Apply gates fn with the injector's fault rules. It is the SERVER-side fault
// seam — a gin middleware or gRPC interceptor wraps its handler call with
// Apply to inject faults into inbound traffic, mirroring how [WrapExecutor]
// gates outbound/client calls. Both directions share the same [Injector] (so
// one governance config drives either), the same [Injector.Gate] sequence and
// the same MaxDuration/MaxAffected guardrails.
//
// nil in => fn runs untouched (zero-config transparency). Apply does NOT
// retry — retry is a client concern; see [Injector.Gate].
func Apply(ctx context.Context, in *Injector, resource string, fn func() error) error {
	if in == nil {
		return fn()
	}
	return in.Gate(ctx, resource, func(context.Context) error { return fn() })
}
