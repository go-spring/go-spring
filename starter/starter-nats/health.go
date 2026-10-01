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

package StarterNats

import (
	"context"
	"errors"

	"go-spring.org/cloud/actuator/health"
)

// errNotConnected is the probe failure. It is a sentinel rather than a wrapped
// client error because the connection state, not a failed operation, is what
// the probe reports.
var errNotConnected = errors.New("nats connection is not established")

// HealthCheck is the module's single connectivity probe: it reports whether the
// connection behind c is currently established. It reads the live state of the
// auto-reconnecting client rather than the outcome of the initial dial, so a
// connection that dropped after startup reports unhealthy until it reconnects.
// It goes straight to the bare *nats.Conn on purpose — a readiness check must
// reflect the backend, not the rate limiter, and must not feed the operation
// metrics or the breaker's statistics — so it touches the unexported conn field
// directly rather than any delegating wrapper method. The startup probe in
// [newConn] and the actuator probe in [NewClientHealth] both delegate here, so
// there is exactly one implementation.
func HealthCheck(ctx context.Context, c *Conn) error {
	if c.conn == nil || !c.conn.IsConnected() {
		return errNotConnected
	}
	return nil
}

// NewClientHealth builds an indicator for a NATS connection. It is registered
// once per configured instance and exported as health.Indicator, so an
// application that also imports starter-actuator gets nats connectivity folded
// into /readiness with no extra wiring. The probe only calls [HealthCheck], so
// it shares the module's one connectivity check.
func NewClientHealth(name string, c *Conn) *health.Indicator {
	return &health.Indicator{Name: "nats:" + name, Probe: func(ctx context.Context) error {
		return HealthCheck(ctx, c)
	}}
}
