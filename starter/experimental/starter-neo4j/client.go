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
// wrapper Neo4j drivers are injected as, plus its lifecycle (Init/Destroy).
// The call-site command seam (Query / RunWithResilience) lives in command.go.
package StarterNeo4j

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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

// Client is the wrapper bean Neo4j drivers are injected as. It
// embeds the neo4j.DriverWithContext interface (so every driver method promotes
// unchanged) and carries the resilience executor built by [Client.ArmGovernance],
// so the policy comes from the governance document and hot-reloads inside the
// executor. newClient returns one and arms it.
//
// The Neo4j seam: the driver's ExecuteQuery is a package-level function (not a
// method on the driver), so there is no transport / dialer / hook to intercept —
// the only viable insertion point is a call-site guard. Applications that call
// [Query] (the instrumented drop-in for neo4j.ExecuteQuery) or
// [RunWithResilience] route through this wrapper's executor automatically when
// resilience is enabled; code that drives sessions directly is untouched (and
// un-protected) unless it calls [RunWithResilience].
type Client struct {
	neo4j.DriverWithContext

	// cfg is the connection config, retained for the resilience service label.
	cfg Config
	// exec is the resilience executor protecting queries, armed by
	// ArmGovernance; nil means "governance off" — [Query] and
	// [RunWithResilience] run their call inline.
	exec resilience.ClientExecutor
}

// ArmGovernance arms the governance-driven resilience stack. It is called by
// the gs wiring with the injected beans — nil when the container has no
// starter-governance, and nil from a standalone caller, both of which mean
// "governance off".
//
// The stack is observe( fault( execFor ) ): fault wraps the resolved executor's
// operation fn so injected failures land INSIDE the retry/breaker loop (and so
// are observed), and the observer behind [Query] / [RunWithResilience] sits
// outermost. inj is nil-safe: with no injector (governance off / fault disabled)
// WrapClientExecutor returns the inner executor unchanged, so the fault layer is a
// transparent pass-through. Resolution is deferred to call time, so the order of
// this arming relative to starter-governance's wiring is irrelevant.
func (o *Client) ArmGovernance(mgr *resilience.Manager, inj *fault.Injector) error {
	// A nil manager is the unwired case — a container without
	// starter-governance (the wiring injects it nullably, so it is nil there
	// too), or a standalone caller that built the driver itself. A fresh
	// unarmed manager is exactly
	// "governance off": every resolve is a pass-through, so [Query] and
	// [RunWithResilience] run their call inline.
	// Normalizing here keeps the rest of this method (and every caller) free of
	// nil branches.
	service := resilience.ServiceLabel("neo4j", o.cfg.ServiceName, o.cfg.URI)
	o.exec = fault.WrapClientExecutor(mgr.ClientExecutorFor("neo4j", service), service, inj)
	return nil
}

// Destroy is the gs destroy method: it closes the resilience executor (if
// armed), stops any discovery watch, and closes the underlying driver.
//
// It is deliberately NOT named Close: the embedded neo4j.DriverWithContext
// already exposes Close(context.Context), and shadowing it with a different
// signature would stop the wrapper from satisfying the interface (and thus from
// being passed to neo4j.ExecuteQuery / [Query]). This teardown is referenced
// explicitly from the gs registration.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return o.DriverWithContext.Close(context.Background())
}
