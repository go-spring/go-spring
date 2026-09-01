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
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
)

// Client is the wrapper bean Neo4j drivers are injected as. It
// embeds the neo4j.DriverWithContext interface (so every driver method promotes
// unchanged) and field-injects the resilience policy via gs.Dync so it
// hot-reloads on config change. newClient returns one; gs field-injects
// Resilience + Observability, then calls Init (InitMethod) to build
// the executor for the call-site guard.
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
	// Both resilience and fault are resolved through neutral seams
	// ([resilience.ExecutorFor] / [fault.InjectorFor]) backed by starter-govern's
	// governance center — so this struct has zero coupling to cloud/governance.
	Observability observe.ObserveConfig `value:"${observability:=}"`

	// cfg is the connection config, retained for the resilience resource label.
	cfg Config
	// resolver is the discovery watch behind a service-name driver (nil when
	// direct/mesh); Close stops it on shutdown.
	resolver *discovery.Resolver
	// exec is the resilience executor protecting queries, resolved via
	// resilience.ExecutorFor; no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("neo4j:<...>") exec scopes
	// limiter/breaker state by.
	resource string
}

// Init is the gs InitMethod: gs field-injects Observability after newClient
// returns, then calls this. It resolves the executor through the neutral
// [resilience.ExecutorFor] seam (backed by starter-govern's governance center
// when imported), wraps it with the process-wide fault injector
// ([fault.InjectorFor], nil-safe), and stores it on the wrapper so [Query] /
// [RunWithResilience] can route through it. When governance is off the resolved
// executor is a transparent no-op.
func (o *Client) Init() error {
	o.resource = resilience.ResourceLabel("neo4j", o.cfg.ServiceName, o.cfg.URI)
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))
	o.exec = resilience.WrapExecutor(exec, "neo4j", o.Observability)
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
	stopLiveResolver(o.resolver)
	return o.DriverWithContext.Close(context.Background())
}
