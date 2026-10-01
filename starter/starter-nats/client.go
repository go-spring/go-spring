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

// client.go is the "resource entity" concept of this starter: the Conn wrapper
// NATS connections are injected as and its lifecycle (NewConn/Destroy). The
// live-health probe lives in health.go. It mirrors starter-memcached's client.go and
// starter-redigo's pool.go: the entity holds the concrete *nats.Conn in an
// unexported field and carries the optional JetStream context and the resilience
// executor, while the per-operation declaration + guard layers live in command.go
// and the raw-client delegations in delegate.go.
package StarterNats

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
)

// Conn wraps a NATS connection together with an optional JetStream context. It
// is the type apps inject (and [Driver.CreateClient] returns); JetStream is
// non-nil only when jetstream.enabled is set, since it is derived from the same
// connection rather than opening a second one.
//
// The raw *nats.Conn is an unexported field, not an embedded one: [NewConn] is
// the only way to build a Conn, so a connection can never exist without its
// identity, and the operation surface (command.go + the delegations in
// delegate.go) is the whole API. There is deliberately no exported accessor for
// the raw connection: that would let a caller bypass the declaration and
// governance layers without it showing up in review. Because nothing is
// promoted, every method the raw *nats.Conn exposed is re-exposed explicitly in
// delegate.go.
//
// Every publish and consume is declared (see [operation]) and routed through the
// resilience executor, which emits the call's span, metrics and access log —
// nats exposes no reject-capable middleware, so the executor is driven at the
// call site rather than threaded in as an interceptor.
type Conn struct {
	// conn is the raw nats connection. Unexported so [NewConn] is the only
	// constructor — see the type doc.
	conn *nats.Conn

	// JetStream is the JetStream context derived from conn, non-nil only when
	// jetstream.enabled is set.
	JetStream jetstream.JetStream

	// exec is the resilience executor every publish and consume runs under; it is
	// also the single emission point for the operation's span, metrics and access
	// log. It is set by [NewConn] from the params bundle it is handed, so a
	// Conn is complete the moment it is built. serviceLabel is the stable
	// per-instance key so the limiter/breaker state is scoped per connection
	// rather than per subject.
	exec         resilience.ClientExecutor
	serviceLabel string
}

// NewConn builds a complete Conn — identity, governance and all — over a
// connected raw client. raw must be ready for use (dialed with its
// options/auth/TLS applied) — it is normally the Driver's product. url is the
// configured server address; it is the identity the resilience service label
// scopes limiter/breaker state by.
//
// params carries the container's facilities (see [cloud.ClientParams]), and is
// applied HERE so a Conn cannot exist half-assembled: there is no Init step, no
// later patching, and nothing the container has to remember to call. A
// hand-built connection passes the zero [cloud.ClientParams]; its executor then
// degrades to [resilience.Unmanaged] — observed, with a one-time warning that no
// protection applies — rather than silently running bare.
//
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the call order relative to the center's wiring is
// irrelevant.
func NewConn(raw *nats.Conn, url string, params cloud.ClientParams) *Conn {
	c := &Conn{
		conn:         raw,
		serviceLabel: resilience.ServiceLabel("nats", url),
	}
	c.exec = params.ExecutorFor("nats", c.serviceLabel)
	return c
}

// Destroy releases the resilience executor (when one is attached) and drains
// the connection, letting in-flight subscriptions finish before the underlying
// socket is closed. Drain closes the connection when done. It is the gs destroy
// method.
func (c *Conn) Destroy() error {
	var execErr error
	if c.exec != nil {
		execErr = c.exec.Close()
	}
	if err := c.conn.Drain(); err != nil {
		return err
	}
	return execErr
}
