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
// its lifecycle (Init/Destroy) and the resource label.
package StarterTdengine

import (
	"database/sql"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

// Client is the wrapper bean TDengine connections are injected as. It embeds
// the *sql.DB pool (so every database/sql method promotes unchanged).
// newClient returns one; gs calls Init (InitMethod) to build the observer +
// executor and arm them on the connection slot the driver installed.
type Client struct {
	*sql.DB

	// cfg is the connection config, retained for the resource label.
	cfg Config
	// slot is the per-statement guard the DefaultDriver installed on every
	// pooled connection; Init arms it.
	slot *clientSlot
	// exec is the resilience executor, resolved via resilience.ExecutorFor;
	// no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("tdengine:<dsn addr>") exec
	// scopes limiter/breaker state by.
	resource string
}

// Init is the gs InitMethod: it builds the per-statement observer and resolves
// the executor through the neutral [resilience.ExecutorFor] seam (backed by
// starter-govern's governance center when imported), wraps it with the
// process-wide fault injector and observe-resilience, and arms both on the
// connection slot. When governance is off the resolved executor is a
// transparent no-op (statements are observe-only).
func (o *Client) Init() error {
	if o.slot != nil {
		o.slot.obs = newDBObserver("tdengine")
	}
	o.resource = resourceLabel(o.cfg)
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))
	exec = resilience.WrapExecutor(exec, "tdengine")
	o.exec = exec
	if o.slot != nil {
		o.slot.exec = exec
		o.slot.resource = o.resource
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

// resourceLabel derives a stable resilience resource key for a client, so
// limiter and breaker state is scoped per TDengine instance rather than per
// statement.
func resourceLabel(c Config) string {
	addr := dsnAddr(c.DSN)
	return resilience.ResourceLabel("tdengine", addr)
}
