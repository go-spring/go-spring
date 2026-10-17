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
// NATS connections are injected as, hollowed into an InnerConn chain — the
// identity layer declares each publish/request, the governance layer runs it
// under the resilience executor, the adapter layer does the wire call. The
// live-health probe lives in health.go; consume keeps a hand-written wrapper
// (see command.go for why its per-delivery pipeline cannot ride the chain).

package StarterNats

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go-spring.org/cloud/governance/label"
	"go-spring.org/cloud/observability"
)

// Conn wraps a NATS connection together with an optional JetStream context. It
// is the type apps inject (and [Driver.CreateClient] returns); JetStream is
// non-nil only when jetstream.enabled is set, since it is derived from the same
// connection rather than opening a second one.
//
// It holds exactly two exported things: the embedded [InnerConn] chain head the
// publish/request commands run through — reorganized by wrapping the head in a
// layer of your own — and [Conn.Conn], the raw connection as a read-only
// handle. [NewConn] is the only way to build one, so a connection can never
// exist without its identity.
type Conn struct {
	// The embedded InnerConn is the chain PublishMsg/PublishMsgContext/
	// RequestGuarded run through: identity over governance over the raw
	// adapter by default, so the default path is always declared and
	// protected. Reorganize it by wrapping the head (see [InnerConn]);
	// build-time only.
	InnerConn

	// Conn is the raw nats connection — the original object, not a wrapper. A
	// read-only handle, never to publish through: that would bypass the
	// chain. Because it is a plain field, every method the raw *nats.Conn
	// exposed is re-exposed explicitly in delegate.go.
	Conn *nats.Conn

	// JetStream is the JetStream context derived from the connection, non-nil
	// only when jetstream.enabled is set.
	JetStream jetstream.JetStream

	// guard is the chain's governance layer, held for Consume: its per-delivery
	// wrapper cannot ride the chain (see command.go), so it runs the executor
	// directly. Unaffected by head reorganization.
	obs *observability.OperationObserver
}

// NewConn builds a complete Conn — identity, governance and all — over a
// connected raw client. raw must be ready to use (dialed with its
// options/auth/TLS applied) — it is normally the Driver's product. url is the
// configured server address; it is the identity the observer's service label
// scopes its signals by. The observer is attached HERE so a Conn cannot exist
// half-assembled. A nats client has no seam for the resilience executor, so
// there is no governance: the observer emits the span, metrics and access log,
// and nothing throttles or retries.
func NewConn(raw *nats.Conn, url string) *Conn {
	obs := observability.NewOperationObserver("nats", label.ServiceLabel("nats", url))
	return &Conn{Conn: raw, InnerConn: NewObsConn(NewRawConn(raw), obs), obs: obs}
}

// Close tears the connection down through its chain — the head's Release(true):
// the executor goes, then the connection drains, letting in-flight
// subscriptions finish before the underlying socket is closed. It is the gs
// destroy method.
func (c *Conn) Close() error { return c.InnerConn.Release(true) }
