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
// connections route statements through the applied executor — plus its lifecycle
// (Destroy) and the service label. The guard lives on the connections the pool
// hands out (installed by the guarded connector), not on this type, so the raw
// *sql.DB is embedded and its whole method set is promoted unchanged.
package StarterTdengine

import (
	"database/sql"
	"database/sql/driver"

	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
)

// Client is the wrapper bean TDengine connections are injected as. It embeds
// the raw *sql.DB pool, so the whole database/sql method set is promoted
// unchanged. That is the right shape here because the pool itself is not
// instrumented: the per-statement guard rides the connections the pool hands out
// (the guardedConnector [NewClient] installs), so this type is a holder, not a
// per-method interceptor. [NewClient] is the only constructor, so a pool can
// never exist without its per-conn guard, and every statement still flows
// through the guard.
type Client struct {
	// *sql.DB is embedded. The pool carries no instrumentation of its own (the
	// guarded connector installed on its connections does), so its methods are
	// promoted rather than re-declared one by one.
	*sql.DB

	// slot is the per-statement guard [NewClient] installed on every pooled
	// connection; the resilience executor is applied on it by [NewClient].
	slot *clientSlot
	// exec is the resilience executor, applied by [NewClient]; it is always
	// non-nil — a client without the container degrades to
	// [resilience.Unmanaged], so statements are always observed.
	exec resilience.ClientExecutor
	// serviceLabel is the resilience service key ("tdengine:<dsn addr>") exec
	// scopes limiter/breaker state by.
	serviceLabel string
}

// NewClient builds a Client over a taosWS connector, fixing its identity,
// installing the per-statement guard and applying governance. It owns the pool
// construction: it creates the shared connection slot the guard lives on, wraps
// connector so every pooled connection routes statements through the slot, opens
// the pool, and applies the pool sizes from c. connector must be a ready-to-use
// driver.Connector (parsed DSN + dialer) — it is normally the Driver's product.
//
// params carries the container's facilities (see [cloud.ClientParams]), and is
// applied HERE so a Client cannot exist half-assembled: there is no Init step,
// no later patching, and nothing the container has to remember to call. It
// resolves the executor once — [cloud.ClientParams.ExecutorFor] — and installs
// it on the slot every pooled connection consults. A hand-built client passes
// the zero [cloud.ClientParams]; its executor then degrades to
// [resilience.Unmanaged] — observed, with a one-time warning that no protection
// applies — rather than silently running bare. The manager resolves its backing
// executor lazily, so the call order relative to the center's wiring is
// irrelevant.
func NewClient(connector driver.Connector, c Config, params cloud.ClientParams) *Client {
	slot := &clientSlot{}
	db := sql.OpenDB(guardedConnector{base: connector, slot: slot})
	db.SetMaxOpenConns(c.MaxOpenConns)
	db.SetMaxIdleConns(c.MaxIdleConns)
	db.SetConnMaxLifetime(c.ConnMaxLifetime)
	cl := &Client{DB: db, slot: slot}
	cl.serviceLabel = serviceLabel(c)
	cl.exec = params.ExecutorFor("tdengine", cl.serviceLabel)
	slot.exec = cl.exec
	return cl
}

// Destroy is the gs destroy method: it closes the resilience executor and the
// connection pool. The executor is always present — [NewClient] installs one
// unconditionally — and its Close is best-effort; the pool's error is returned.
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
