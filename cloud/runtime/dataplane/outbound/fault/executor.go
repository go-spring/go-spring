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

	kfault "go-spring.org/cloud/runtime/dataplane/kernel/fault"
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
)

// executor.go is the OUTBOUND shell: [Client] is the state handle a caller (or a
// custom adapter) fires through, and [faultExecutor] is the [chain.Executor] a
// client wraps itself in. Both read their OWN kernel pointer; neither consults
// the manager on the call path.

// Client is one outbound shell's state: the kernel it currently runs and the
// load-test propagator it was created with. Apart from the atomic kernel pointer
// — which [ClientManager.Refresh] pushes into — it is immutable, so a call
// reads it once and never asks the manager anything.
//
// Create one with [ClientManager.NewClient] (a custom adapter driving its own
// call loop) or implicitly with [ClientManager.WrapClientExecutor]. Close it when
// the consumer goes away, so the manager stops pushing to a state that is gone.
type Client struct {
	*kfault.Shell
}

// Fire decides whether one outbound attempt should be set on fire, and applies
// the configured latency. It is the CLIENT side's entry into the shared sequence
// — the one [faultExecutor] uses per attempt, and the one a custom adapter that
// drives its own call loop can use instead of wrapping an executor.
//
// It does NOT call anything: the returned error is the "may I proceed?" answer —
// nil means run the real call, anything else means return it instead (an
// injected failure, or the context error when the injected latency was cancelled
// mid-sleep). [kfault.Decision] says what happened either way.
//
// The outbound side is fixed by which shell this is, so a caller cannot aim a
// fire at the inbound config; its inbound counterpart is
// [go-spring.org/cloud/runtime/dataplane/inbound/fault.Server.Fire].
func (c *Client) Fire(ctx context.Context, t kfault.Target) (kfault.Decision, error) {
	return c.Shell.Fire(ctx, t)
}

// faultExecutor is the [chain.Executor] [ClientManager.WrapClientExecutor]
// returns: an outbound shell over the executor it delegates to.
type faultExecutor struct {
	client *Client
	inner  chain.Executor
}

// WrapClientExecutor returns a chain.Executor that injects faults into fn before
// delegating to inner. It is the manager's object-creation half, mirroring
// resilience.Manager.ExecutorFor: the shell it returns is immutable and lives as
// long as the client does, while the firing state it reads is its OWN kernel,
// pushed by [ClientManager.Refresh].
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
// it is consulted only at construction, to create and register the shell. It must
// be non-nil: the bean is always present (its constructor is REQUIRED — an absent
// bean fails startup), and with no rule armed the shell is a transparent
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
func (m *ClientManager) WrapClientExecutor(inner chain.Executor) chain.Executor {
	return &faultExecutor{client: m.NewClient(), inner: inner}
}

// Execute wraps fn so each attempt is faulted per the shell's current kernel
// before the real executor sees it. An injected latency sleeps first (cancellable
// via the attempt context); if the sleep is cancelled the context error is
// returned so the executor's budget/timeout logic reacts rather than retrying
// blindly.
//
// The kernel is loaded ONCE per call, at Execute entry: every attempt of the call
// sees the same document, which is what makes a retry's outcome comparable to the
// first try. A push landing mid-call takes effect on the next call.
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
	snap := e.client.Snapshot()
	return e.inner.Execute(ctx, call, rec, func(attemptCtx context.Context) error {
		d, err := snap.Decide(attemptCtx, t)
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
			// with it (the `resilience.injected` attributes).
			rec.SetFault(&chain.Injection{
				Kind:  d.Kind,
				Rule:  d.Rule,
				Delay: d.Delay,
			})
		}
		if err != nil {
			// An injected failure, or the attempt's context error when an injected
			// latency was cancelled: the real fn must not run.
			return err
		}
		return fn(attemptCtx)
	})
}

// Close releases the shell and forwards to the inner executor: the shell stops
// receiving pushes, then the executor it wrapped is closed.
func (e *faultExecutor) Close() error {
	e.client.Close()
	return e.inner.Close()
}
