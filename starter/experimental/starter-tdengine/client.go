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
// wrapper TDengine connections are injected as — a *sql.DB pool whose
// connections route statements through the armed executor + observer — plus
// its lifecycle (Init/Destroy) and the service label.
package StarterTdengine

import (
	"database/sql"

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

// Client is the wrapper bean TDengine connections are injected as. It embeds
// the *sql.DB pool (so every database/sql method promotes unchanged).
// newClient returns one, arms its governance stack through
// [Client.ArmGovernance], and lets gs call Init (InitMethod) to build the
// per-statement observer on the connection slot the driver installed.
type Client struct {
	*sql.DB

	// cfg is the connection config, retained for the service label.
	cfg Config
	// slot is the per-statement guard the DefaultDriver installed on every
	// pooled connection; Init arms it.
	slot *clientSlot
	// exec is the resilience executor, armed by ArmGovernance; nil means
	// "governance off" — statements are observe-only.
	exec resilience.ClientExecutor
	// service is the resilience service key ("tdengine:<dsn addr>") exec
	// scopes limiter/breaker state by.
	service string
}

// Init is the gs InitMethod: it builds the per-statement observer and arms it on
// the connection slot. Governance (the resilience executor) is armed separately
// by [Client.ArmGovernance], which the gs wiring calls with the injected beans —
// see that method for why it is not part of this lifecycle hook.
func (o *Client) Init() error {
	if o.slot != nil {
		o.slot.obs = newDBObserver()
	}
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
// of those mechanisms are in play. The slot observer [Client.Init] arms stays
// innermost and times each statement. inj is nil-safe:
// with no injector (governance off / fault disabled) WrapClientExecutor returns the
// inner executor unchanged, so the fault layer is a transparent pass-through.
// Resolution is deferred to call time, so the order of this arming relative to
// starter-governance's wiring is irrelevant.
//
// Arming the slot here (rather than in Init) is what keeps the pool's own
// construction — done by the Driver — free of a governance dependency, and lets
// a custom Driver's client be governed without changing the Driver interface.
func (o *Client) ArmGovernance(mgr *resilience.Manager, inj *fault.Injector) error {
	// A nil manager is the unwired case — a container without
	// starter-governance (the wiring injects it nullably, so it is nil there
	// too), or a standalone caller that built the pool itself. A fresh
	// unarmed manager is exactly
	// "governance off": every resolve is a pass-through, so the slot runs
	// statements inline and they are observe-only.
	// Normalizing here keeps the rest of this method (and every caller) free of
	// nil branches.
	o.service = serviceLabel(o.cfg)
	o.exec = fault.WrapClientExecutor(mgr.ClientExecutorFor("tdengine", o.service), o.service, inj)
	if o.slot != nil {
		o.slot.exec = o.exec
	}
	return nil
}

// Destroy is the gs destroy method: it closes the resilience executor (if
// armed) and the connection pool.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return o.DB.Close()
}

// serviceLabel derives a stable resilience service key for a client, so
// limiter and breaker state is scoped per TDengine instance rather than per
// statement.
func serviceLabel(c Config) string {
	addr := dsnAddr(c.DSN)
	return resilience.ServiceLabel("tdengine", addr)
}
