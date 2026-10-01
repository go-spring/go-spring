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
// wrapper Neo4j drivers are injected as, plus its lifecycle (construction/
// Destroy). The call-site command seam (Query / RunWithResilience) lives in
// command.go.
package StarterNeo4j

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
)

// Client is the wrapper bean Neo4j drivers are injected as. It embeds the raw
// neo4j.DriverWithContext, so it satisfies that interface by promotion: it can
// be handed to neo4j.ExecuteQuery and to this starter's own [Query] /
// [RunWithResilience] seam unchanged. [NewClient] is the only way to build a
// Client, so a driver can never exist without its identity (the service label),
// and the resilience executor [NewClient] builds from the governance bundle it
// is handed is the only extra capability it carries.
//
// The wrapper is a pure holder here, which is why the driver is embedded rather
// than re-declared method by method: neo4j-go-driver's ExecuteQuery is a
// package-level function (not a method on the driver), and the driver exposes
// no transport / dialer / hook to intercept — so the operation's identity is
// declared by the call-site free functions (see command.go) and the signals are
// emitted by the resilience layer, never by this type's methods.
// Applications that call [Query] (the instrumented drop-in for
// neo4j.ExecuteQuery) or [RunWithResilience] route through this wrapper's
// executor automatically when resilience is enabled; code that drives sessions
// directly is untouched (and un-protected) unless it calls [RunWithResilience].
type Client struct {
	// neo4j.DriverWithContext is embedded. The driver carries no per-instance
	// instrumentation (the declaration is a call-site concern and the guard is a
	// call-site seam), so its method set is promoted unchanged.
	neo4j.DriverWithContext

	// serviceLabel is the resilience service key ("neo4j:<service-name or
	// uri>") exec scopes limiter/breaker state by. Fixed by [NewClient].
	serviceLabel string

	// exec is the resilience executor protecting queries, fixed by [NewClient]
	// from the governance bundle it is handed. On a client built there it is
	// never nil — a zero bundle degrades to resilience.Unmanaged (observed, with
	// a one-time warning) rather than silently running bare. A zero Client built
	// by hand (a test) leaves it nil, and [Query] / [RunWithResilience] then run
	// their call inline.
	exec resilience.ClientExecutor
}

// NewClient builds a complete Client — identity and governance both — over a
// live raw driver, handing back a wrapper ready to use. client must be ready
// for use — it is normally the Driver's product.
//
// The declaration layer for neo4j is the per-call [StartSpan] / [Query] seam
// (observe.go): the driver exposes no interception point, so the operation's
// identity is carried on the ctx by the free-function call path rather than by
// any per-instance object. There is therefore no Init step — building a Client
// and initializing it are the same act, so the container has no lifecycle hook
// to register and no way to hand out a half-built client.
//
// params carries the container's facilities (see [cloud.ClientParams]), applied
// HERE so a Client cannot exist half-assembled: there is no later patching and
// nothing the wiring has to remember to call. A hand-built client passes the
// zero [cloud.ClientParams]; its executor then degrades to resilience.Unmanaged —
// observed, with a one-time warning that no protection applies — rather than
// silently running bare. The manager's ClientExecutorFor resolves its backing
// executor lazily, on each Execute, so the call order relative to
// the center's wiring is irrelevant.
func NewClient(client neo4j.DriverWithContext, cfg Config, params cloud.ClientParams) *Client {
	c := &Client{
		DriverWithContext: client,
	}
	c.serviceLabel = resilience.ServiceLabel("neo4j", cfg.ServiceName, cfg.URI)
	c.exec = params.ExecutorFor("neo4j", c.serviceLabel)
	return c
}

// Destroy releases the resilience executor and closes the underlying driver. It
// is the gs destroy method.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return o.DriverWithContext.Close(context.Background())
}
