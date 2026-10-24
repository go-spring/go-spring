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

package runtime

import (
	"context"
	"time"

	"go-spring.org/cloud/runtime/dataplane/inbound/admission"
	"go-spring.org/cloud/runtime/dataplane/inbound/fault"
	"go-spring.org/cloud/runtime/dataplane/inbound/observe"
)

// InboundFor returns the pair of inbound objects one route's middleware drives,
// wired together: the observer records the request plane, while the admission
// publishes its own state. It replaces the old ServerExecutorFor — inbound is no longer a chain
// (a governed request has two hook points a chain cannot span), so what a server
// gets is two peer objects, not one executor.
//
// It is the same single composition point the outbound clients reach: the admission
// authority supplies the governance object and this runtime (the only importer
// of both) supplies the observer, so no middleware wires the observe object to
// the admission itself and no other package needs to know both. The inbound fault
// authority is folded in too, innermost, so a server has ONE inbound seam.
func (r *Runtime) InboundFor(system, label string) (*Inbound, error) {
	return InboundOver(system, label, r.guardServer, r.injServer)
}

// InboundOver builds the inbound pair over an explicit admission authority — the
// seam for a caller that holds the manager rather than a runtime (a filter
// library with no bean to inject into, a test). mgr MUST be non-nil: an invalid
// construction is refused, never silently wrapped.
//
// inj is optional: when present, the inbound fault stage becomes part of the
// admission (innermost, after the breaker gate), so ordering and breaker accounting
// are fixed in one place. A caller with no injector gets governance without
// fault.
func InboundOver(system, label string, mgr *admission.Manager, inj ...*fault.ServerManager) (*Inbound, error) {
	if mgr == nil {
		panic("runtime: InboundOver requires a non-nil admission.Manager")
	}
	var opts []admission.Option
	if len(inj) > 0 && inj[0] != nil {
		opts = append(opts, admission.WithFault(fault.Stage(system, inj[0].NewServer())))
	}
	obs := observe.NewServerObserver(system, label)
	g, err := mgr.GuardFor(admission.Identity{System: system, Service: label}, opts...)
	if err != nil {
		return nil, err
	}
	return &Inbound{observe: obs, govern: g}, nil
}

// Refusal is a rejected request's error. It carries the admission's neutral verdict
// so an adapter maps the protocol word without importing admission's internals.
type Refusal struct {
	result admission.Result
}

func (r *Refusal) Error() string {
	if err := r.result.Err(); err != nil {
		return err.Error()
	}
	return "admission: " + r.result.Verdict().String()
}

// Unwrap exposes the underlying sentinel for errors.Is.
func (r *Refusal) Unwrap() error { return r.result.Err() }

// Verdict is why the request was refused.
func (r *Refusal) Verdict() admission.Verdict { return r.result.Verdict() }

// RetryAfter is how long the caller is asked to wait, when known.
func (r *Refusal) RetryAfter() time.Duration { return r.result.RetryAfter() }

// Result returns the full admission result.
func (r *Refusal) Result() admission.Result { return r.result }

// Inbound is a middleware's handle on one route's inbound governance and
// observation. It holds NO per-request state and NO domain state — the state
// lives on the [admission.Guard] and on
// [observe.ServerObserver]/[observe.Observation]. Its only job is to
// fix the layer order — observation outermost, admission inside — so an adapter
// cannot get it wrong and a rejected request is still traced.
type Inbound struct {
	observe *observe.ServerObserver
	govern  admission.Guard
}

// Run is the PREFERRED form for a protocol whose handler can be wrapped in one
// closure. It opens the span, admits inside it, and runs fn — the handler, with
// fault innermost — last, then settles. It returns fn's error, or a [Refusal]
// when the request never reached fn.
//
// A handler that must record the late half of its operation (the response status
// code) uses the [Inbound.Before]/[Inbound.After] form instead, whose [Hooks]
// exposes the observation.
func (in *Inbound) Run(ctx context.Context, op observe.ServerOperation, fn func(context.Context) error) error {
	return in.observe.Run(ctx, op, func(ctx context.Context, _ *observe.Observation) error {
		return in.run(ctx, op, fn)
	})
}

// GovernedRun runs the request under governance only, with no observation. It is
// the seam for a server that has turned its own observability off but still
// wants admission control.
func (in *Inbound) GovernedRun(ctx context.Context, op observe.ServerOperation, fn func(context.Context) error) error {
	return in.run(ctx, op, fn)
}

// run is the shared admission + completion path.
func (in *Inbound) run(ctx context.Context, op observe.ServerOperation, fn func(context.Context) error) (err error) {
	r, done := in.govern.Admit(ctx, admission.Request{
		Action:   op.Action,
		Metadata: op.Metadata,
	})
	if !r.Admitted() {
		return &Refusal{result: r}
	}

	ctx, cancel := withBudget(ctx, r.Timeout())
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			done()
			panic(p)
		}
	}()
	err = fn(ctx)
	done()
	return err
}

// Before is the SPLIT form, for a protocol whose request cannot be wrapped in one
// closure. It opens the span FIRST, then admits — so a rejection is still
// observed — and on rejection closes the span and returns the [Refusal]. On
// admission it returns the [Hooks] to settle with [Inbound.After].
func (in *Inbound) Before(ctx context.Context, op observe.ServerOperation) (*Hooks, error) {
	ctx, ob := in.observe.Begin(ctx, op)
	r, done := in.govern.Admit(ctx, admission.Request{
		Action:   op.Action,
		Metadata: op.Metadata,
	})
	if !r.Admitted() {
		ref := &Refusal{result: r}
		in.observe.End(ob, ref)
		return nil, ref
	}
	runCtx, cancel := withBudget(ctx, r.Timeout())
	return &Hooks{ctx: runCtx, ob: ob, done: done, cancel: cancel}, nil
}

// After settles an attempt begun by [Inbound.Before]: the admission first (it
// releases the bulkhead slot), then the observer.
func (in *Inbound) After(h *Hooks, err error) {
	if h == nil {
		return
	}
	h.done()
	if h.cancel != nil {
		h.cancel()
	}
	in.observe.End(h.ob, err)
}

// Hooks carries one split-form request's tokens: the context the handler runs
// under, the observation and the admission's completion. The state lives on those
// objects, not here.
type Hooks struct {
	ctx    context.Context
	ob     *observe.Observation
	done   admission.Done
	cancel context.CancelFunc
}

// Context is the context the handler should run under — the span and the route's
// handling budget applied.
func (h *Hooks) Context() context.Context { return h.ctx }

// Observation returns the request's observation, for recording the response half.
func (h *Hooks) Observation() *observe.Observation { return h.ob }

// Observe returns the observer alone, for a caller that wants the observation
// signals without governance.
func (in *Inbound) Observe() *observe.ServerObserver { return in.observe }

// withBudget applies the policy's handling budget when one is configured.
func withBudget(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d > 0 {
		return context.WithTimeout(ctx, d)
	}
	return ctx, func() {}
}
