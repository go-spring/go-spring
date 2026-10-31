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

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// executor.go is the OUTBOUND seam: [Client] is the entry point a caller (or a
// custom adapter) fires through, and [faultExecutor] is the [chain.Executor] a
// client wraps itself in. Both load the authority's published kernel pointer;
// neither takes its lock on the call path.

// Client is the outbound entry point a caller fires through: it holds the
// outbound authority and exposes only Fire, so the caller cannot aim a fire at
// the inbound config or reach the authority's Refresh/Config.
//
// Create one with [Manager.NewClient] (a custom adapter driving its own
// call loop) or implicitly with [Manager.WrapClientExecutor].
type Client struct {
	mgr *kfault.Manager
}

// Fire decides whether one outbound attempt should be set on fire, and applies
// the configured latency. It is the CLIENT side's entry into the shared sequence
// — the one [faultExecutor] uses per attempt, and the one a custom adapter that
// drives its own call loop can use instead of wrapping an executor.
//
// It does NOT call anything: the returned error is the "may I proceed?" answer —
// nil means run the real call, anything else means return it instead (an
// injected failure, or the context error when the injected latency was cancelled
// mid-sleep). [kfault.Result] says what happened either way.
//
// t is the call being fired at, as the adapter names it. It is the adapter's to
// BUILD, traffic class included: construct it with [chain.NewOperation] (which
// records the call's traffic class from the context) or put "traffic" into
// [kfault.Target.Metadata] by hand. This seam adds nothing — a Target without
// that entry simply does not match a rule that names match.traffic.
//
// The outbound side is fixed by which authority this is, so a caller cannot aim
// a fire at the inbound config; its inbound counterpart is
// [go-spring.org/cloud/governance/dataplane/inbound/fault.Server.Fire].
func (c *Client) Fire(ctx context.Context, t kfault.Target) (kfault.Result, error) {
	return c.mgr.Fire(ctx, t)
}

// faultExecutor is the [chain.Executor] [Manager.WrapClientExecutor]
// returns: an outbound seam over the executor it delegates to.
type faultExecutor struct {
	client *Client
	inner  chain.Executor
}

// WrapClientExecutor returns a chain.Executor that injects faults into fn before
// delegating to inner. It is the manager's object-creation half, mirroring
// resilience.Manager.ExecutorFor: the wrapper it returns lives as long as the
// client does, while the firing state it reads is the authority's current
// kernel, published by [Manager.Refresh].
//
// The injection happens INSIDE inner's retry loop: the wrapped fn either
// returns the injected error (so the real executor retries, the breaker counts
// the failure, and observe records the outcome) or calls the real fn. This is
// what makes "setting fire" validate the resilience stack — a fault flows
// through retry/breaker/timeout/Fallback exactly as a real downstream failure
// would, rather than short-circuiting at the boundary where none of those
// mechanisms are in play.
//
// The manager is the outbound authority the caller received from the container;
// it is consulted only at construction, to wire the entry point. It must be
// non-nil: the bean is always present (its constructor is REQUIRED — an absent
// bean fails startup), and with no rule armed the seam is a transparent
// pass-through anyway.
//
// The service a fault rule matches is the call's OWN: [chain.Operation.Service],
// which the client states when it builds the call (see [kfault.Target]). This
// adapter takes no label of its own — a call that names no service matches only
// the rules that name none, and quietly borrowing a label here would make a
// mislabelled call look correctly targeted. inner is the executor for the same
// service, so a fault rule and a resilience rule name the same thing.
//
// inner must be non-nil: every caller builds it through the
// resilience/observability stack (never nil).
func (m *Manager) WrapClientExecutor(inner chain.Executor) chain.Executor {
	return &faultExecutor{client: m.NewClient(), inner: inner}
}

// Execute wraps fn so each attempt is faulted per the authority's current kernel
// before the real executor sees it. An injected latency sleeps first (cancellable
// via the attempt context); if the sleep is cancelled the context error is
// returned so the executor's budget/timeout logic reacts rather than retrying
// blindly.
//
// The kernel is read at each attempt: a push landing mid-attempt does not affect
// that attempt, and takes effect on the next one.
func (e *faultExecutor) Execute(ctx context.Context, call chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	// The adapter translates: the client-shaped call description collapses into
	// the three coordinates a rule matches (see [kfault.Target]). Service is
	// taken as the call states it — no fallback, because a fallback would hide a
	// caller that forgot to name the service it is calling.
	t := kfault.Target{
		Service:  call.Service,
		Action:   call.Request.Action,
		Metadata: call.Request.Metadata,
	}
	return e.inner.Execute(ctx, call, rec, func(attemptCtx context.Context) error {
		d, err := e.client.Fire(attemptCtx, t)
		if d.Fired() {
			// The record is CALL-level (one Recorder per call, shared by every
			// attempt), so the write must still happen per attempt: a first attempt
			// that fired and a second that did not must leave the fire on the
			// record. Writing once after Execute returns, from the final decision
			// alone, would lose exactly that case — the decision that ended the call
			// is the one that did NOT fire.
			//
			// The last fire wins, which is the answer a reader of the rounds would
			// derive. Observe reads it back and tags the span and the access line
			// with it (the `resilience.injected` attributes). At most one rule
			// governed the attempt, so the record names exactly that rule.
			rec.SetFault(&d)
		}
		if err != nil {
			// An injected failure, or the attempt's context error when an injected
			// latency was cancelled: the real fn must not run.
			return err
		}
		return fn(attemptCtx)
	})
}

// Close forwards to the inner executor. There is no per-consumer state left to
// release: the outbound authority is process-lived.
func (e *faultExecutor) Close() error {
	return e.inner.Close()
}
