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

// executor.go is the OUTBOUND seam: [faultExecutor] is the [chain.Executor] a
// client wraps itself in. It loads the authority's published kernel pointer and
// never takes its lock on the call path.

// faultExecutor is the [chain.Executor] [Manager.WrapClientExecutor]
// returns: the innermost layer of the chain, injecting into each attempt of the
// executor it delegates to.
type faultExecutor struct {
	mgr  *kfault.Manager
	next chain.Executor
}

// Execute injects into the fn it hands to next, so every attempt of the rest of
// the chain is faulted before the real call is reached. An injected latency
// sleeps first (cancellable via the attempt context); if the sleep is cancelled
// the context error is returned so the executor's budget/timeout logic reacts
// rather than retrying blindly.
//
// Handing next a WRAPPED fn (rather than doing the fire before delegating) is
// what puts injection inside next's attempt loop: when next is a governance
// executor, its retry loop calls this fn once per attempt. When next is nil,
// [chain.Next] calls the wrapped fn directly, so this layer is the terminal and
// the fire happens immediately before the real call.
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
	return chain.Next(e.next, ctx, call, rec, func(attemptCtx context.Context) error {
		d, err := e.mgr.Fire(attemptCtx, t)
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

// Close forwards to the next layer. There is no per-consumer state left to
// release: the outbound authority is process-lived.
func (e *faultExecutor) Close() error {
	if e.next == nil {
		return nil
	}
	return e.next.Close()
}
