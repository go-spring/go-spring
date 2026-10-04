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

// entity.go is the "resource entity" concept of this starter: the Client
// wrapper RabbitMQ connections are injected as, hollowed into an InnerPublisher
// chain — the identity layer declares each publish, the governance layer runs it
// under the resilience executor, the adapter layer injects the W3C trace context
// and makes the wire call. This wrapper replaces the earlier per-connection
// guard registry: the chain travels with the client, not a package-global map.
package StarterRabbitMQ

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/governance"
)

// Client wraps an AMQP connection. It holds exactly two exported things: the
// embedded [InnerPublisher] chain head the publishes run through —
// reorganized by wrapping the head in a layer of your own — and
// [Client.Conn], the raw connection as a read-only handle (exchanges, custom
// routing, publisher confirms and other AMQP features the messaging driver
// does not model). [NewClient] is the only way to build one.
type Client struct {
	// The embedded InnerPublisher is the chain Publish runs through: identity
	// over governance over the raw adapter by default, so the default path is
	// always declared and protected. Reorganize it by wrapping the head (see
	// [InnerPublisher]); build-time only.
	InnerPublisher

	// Conn is the raw AMQP connection — the original object, not a wrapper. A
	// read-only handle for the AMQP features outside the chain; publishing
	// through it bypasses the chain.
	Conn *amqp.Connection

	// guard is the chain's governance layer, held for the messaging driver's
	// per-delivery consume path (whose pipeline inverts the chain's
	// composition — see [Client.execute]). Unaffected by head reorganization.
	guard *GuardPublisher
}

// NewClient builds a complete Client — identity, governance and all — over a
// connected raw connection. serviceLabel is the governance label the executor
// scopes limiter/breaker state by; center is the governance center the
// container injects (nil reads unarmed authorities — exactly "governance
// off", whose executor is a pass-through).
func NewClient(conn *amqp.Connection, serviceLabel string, center *governance.Center) *Client {
	exec := fault.WrapClientExecutor(center.Resilience().ClientExecutorFor("rabbitmq", serviceLabel), serviceLabel, center.Fault())
	guard := &GuardPublisher{exec: exec}
	return &Client{Conn: conn, InnerPublisher: NewObsPublisher(guard), guard: guard}
}

// Close tears the client down: the governance layer's executor goes first,
// then the connection closes (which also closes the notifier channels and
// drains the log-bridging goroutines). It is the gs destroy method.
func (c *Client) Close() error {
	if c.guard != nil && c.guard.exec != nil {
		_ = c.guard.exec.Close()
	}
	return c.Conn.Close()
}

// execute routes a per-delivery consume call through the governance layer's
// executor, and otherwise runs it inline. The consume pipeline (extract trace,
// declare, execute) is per-delivery and inverted relative to the chain's
// outside-in composition — the same reason nats's Consume keeps a hand-written
// wrapper — so it uses this seam instead of the chain.
func (c *Client) execute(ctx context.Context, call func(context.Context) error) error {
	if c.guard == nil || c.guard.exec == nil {
		return call(ctx)
	}
	return c.guard.exec.Execute(ctx, call)
}
