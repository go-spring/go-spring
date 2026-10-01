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

// client.go is the "resource entity" concept of this starter: the Client
// wrapper Cassandra sessions are injected as, plus its lifecycle (Init/
// Destroy), the service label, and the guard seam that routes statements
// through the resilience executor + observer. gocql exposes no reject-capable
// middleware, so the guard rides the guarded *Query wrapper every
// Client.Query/Client.Bind call returns (query.go) — coverage of the normal
// statement path is transparent, no opt-in helper required.
package StarterCassandra

import (
	"context"

	"github.com/gocql/gocql"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is spring.governance.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// Client is the wrapper bean Cassandra sessions are injected as. It embeds
// the concrete *gocql.Session (so Query/Iter/Close and friends promote
// unchanged). newClient returns one, arms its governance stack through
// [Client.ArmGovernance], and lets gs call Init (InitMethod) to build the
// observer.
type Client struct {
	*gocql.Session

	// cfg is the connection config, retained for the service label.
	cfg Config
	// exec is the resilience executor protecting statements, armed by
	// ArmGovernance; nil means "governance off" — the guard runs the call
	// inline and the path is observe-only.
	exec resilience.ClientExecutor
	// service is the resilience service key ("cassandra:<hosts>") exec
	// scopes limiter/breaker state by.
	service string
	// obs is the observer behind the guarded statement path (see observe.go).
	obs *dbObserver
}

// Init is the gs InitMethod: it builds the observer behind the guarded
// statement path. Governance (the resilience executor) is armed separately by
// [Client.ArmGovernance], which the gs wiring calls with the injected beans —
// see that method for why it is not part of this lifecycle hook.
func (o *Client) Init() error {
	o.obs = newDBObserver()
	return nil
}

// ArmGovernance arms the governance-driven resilience stack. It is called by
// the gs wiring with the injected beans — nil when the container has no
// starter-governance, and nil from a standalone caller, both of which mean
// "governance off".
//
// The stack is fault( observe( core ) ): the executor [Manager.ClientExecutorFor]
// returns already carries the resilience observe layer around the
// limiter/breaker/retry core, and fault.WrapClientExecutor wraps the operation fn that
// executor runs — so an injected fault flows through retry/breaker/timeout
// exactly as a downstream failure would, instead of short-circuiting where none
// of those mechanisms are in play. The observer [Client.Init] builds stays
// innermost and times the statement itself. inj is
// nil-safe: with no injector (governance off / fault disabled) WrapClientExecutor
// returns the inner executor unchanged, so the fault layer is a transparent
// pass-through. Resolution is deferred to call time, so the order of this arming
// relative to starter-governance's wiring is irrelevant.
func (o *Client) ArmGovernance(mgr *resilience.Manager, inj *fault.Injector) error {
	// A nil manager is the unwired case — a container without
	// starter-governance (the wiring injects it nullably, so it is nil there
	// too), or a standalone caller that built the client itself. A fresh
	// unarmed manager is exactly
	// "governance off": every resolve is an observe-only pass-through.
	// Normalizing here keeps the rest of this method (and every caller) free of
	// nil branches.
	o.service = resilience.ServiceLabel("cassandra", o.cfg.Hosts[0])
	o.exec = fault.WrapClientExecutor(mgr.ClientExecutorFor("cassandra", o.service), o.service, inj)
	return nil
}

// Destroy is the gs destroy method: it closes the resilience executor (if
// armed) and the session.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	o.Session.Close()
	return nil
}

// guard routes call through the client's executor (when armed) and wraps it
// in an observation (when an observer is armed). It is the single guarded
// seam every statement path rides — the Query/Bind wrappers in query.go and
// the Exec alias below.
func (o *Client) guard(ctx context.Context, op, stmt string, call func(context.Context) error) error {
	if o.obs != nil {
		inner := call
		call = func(ctx context.Context) error {
			ctx, sp := o.obs.Start(ctx, op, stmt)
			err := inner(ctx)
			sp.End(err)
			return err
		}
	}
	if o.exec == nil {
		return call(ctx)
	}
	return o.exec.Execute(ctx, call)
}

// Exec executes a statement synchronously through the guarded path. It is now
// a thin alias over the guarded Query wrapper (kept for callers written
// against the earlier opt-in helper); Query(...).Exec() is the same call.
// For iterators and paging use Query(...).Iter(), which is guarded too.
func (o *Client) Exec(ctx context.Context, stmt string, values ...any) error {
	return o.Query(stmt, values...).WithContext(ctx).Exec()
}
