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
// NATS connections are injected as, plus its lifecycle (destroy = Drain) and the
// live-health probe. It mirrors starter-redigo's pool.go: the entity embeds the
// concrete *nats.Conn and carries the optional JetStream context, the observe
// observers, and the resilience executor, while the per-operation observe +
// guard layers live in command.go.
package StarterNats

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go-spring.org/cloud/governance/resilience"
)

// Conn wraps a NATS connection together with an optional JetStream context.
// The embedded *nats.Conn lets callers use Publish/Subscribe/Request directly on
// the bean; JetStream is non-nil only when jetstream.enabled is set, since it is
// derived from the same connection rather than opening a second one.
//
// When governance is enabled the opt-in PublishGuarded(ctx, …) and
// RequestGuarded methods route the call through a rate-limiter / circuit-
// breaker executor; the plain Publish/Request remain untouched. nats exposes no
// reject-capable middleware, so the guard lives at the call site — callers pick
// per-invocation whether they want protection.
type Conn struct {
	*nats.Conn
	JetStream jetstream.JetStream

	// pubObs/subObs drive the instrumentation (trace+metric+log) for publishes
	// and consumes. nil-safe: when nil the instrumented methods delegate
	// unchanged.
	pubObs *observer
	subObs *observer

	// exec is nil unless governance is enabled; when set, the guarded
	// methods route through it. resource is the stable per-instance key so the
	// limiter/breaker state is scoped per connection rather than per subject.
	exec     resilience.Executor
	resource string
}

// Healthy reports whether the connection is currently established. It reflects
// the live state of the auto-reconnecting client, so callers (health probes,
// readiness endpoints) can query it at any time rather than relying only on the
// connection-event logs.
func (c *Conn) Healthy() bool {
	return c.Conn != nil && c.Conn.IsConnected()
}

// destroyConn drains the connection, letting in-flight subscriptions finish
// before the underlying socket is closed. Drain closes the connection when done.
// When a resilience executor is attached its Close releases any background
// resources of a production driver.
func destroyConn(conn *Conn) error {
	var execErr error
	if conn.exec != nil {
		execErr = conn.exec.Close()
	}
	if err := conn.Drain(); err != nil {
		return err
	}
	return execErr
}
