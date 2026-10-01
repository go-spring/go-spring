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

package StarterCassandra

import (
	"context"

	"go-spring.org/cloud/actuator/health"
)

// HealthCheck probes a Cassandra client with the cheapest read-only round trip:
// it scans system.local on the contact point, one round trip that verifies
// protocol, auth and cluster state. It is the module's single health
// implementation — the Actuator probe ([NewClientHealth]) and the startup probe
// in the constructor both delegate to it.
//
// It goes straight to the raw session on purpose: a readiness check must
// reflect the backend, not the rate limiter, and must not feed the operation
// metrics or the breaker's statistics.
func HealthCheck(ctx context.Context, c *Client) error {
	var release string
	return c.session.Query("SELECT release_version FROM system.local").WithContext(ctx).Scan(&release)
}

// NewClientHealth builds an indicator for a Cassandra client. It is registered
// once per configured instance and exported as health.Indicator, so an
// application that also imports starter-actuator gets Cassandra readiness
// folded into /readiness with no extra wiring. The probe delegates to
// [HealthCheck], the module's single health implementation.
func NewClientHealth(name string, c *Client) *health.Indicator {
	return &health.Indicator{Name: "cassandra:" + name, Probe: func(ctx context.Context) error {
		return HealthCheck(ctx, c)
	}}
}
