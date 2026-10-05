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

// health.go builds this starter's health.Indicator beans: one per configured
// instance, exported so an application that also imports starter-actuator gets
// Redis readiness folded into /readiness with no extra wiring.

package StarterGoRedis

import (
	"context"

	"go-spring.org/cloud/actuator/health"
)

// HealthCheck probes a Redis client with the cheapest read-only round trip: a
// PING to the backend. It is the module's single health implementation — the
// Actuator probe ([NewClientHealth]) and the startup probe in the constructors
// both delegate to it.
//
// Unlike the other client starters this probe cannot bypass the instrumentation:
// the declaration and resilience layers are go-redis hooks attached to the raw
// client the wrapper embeds, so a PING issued through the embedded client
// necessarily rides them (it declares nothing — PING is a skip op — so the
// resilience layer reports it as an undeclared call). That is inherent to how
// go-redis exposes its extension point, not a choice made here.
func HealthCheck(ctx context.Context, c *Client) error {
	return c.Ping(ctx).Err()
}

// NewClientHealth builds an indicator for a Redis client. Single, sentinel and
// cluster modes all hand in the same [Client] wrapper (it hides the raw
// *redis.Client / *redis.ClusterClient difference), so one constructor covers
// every topology.
//
// It is registered once per configured instance and exported as
// health.Indicator, so an application that also imports starter-actuator gets
// Redis readiness folded into /readiness with no extra wiring. The probe
// delegates to [HealthCheck], the module's single health implementation.
func NewClientHealth(name string, c *Client) *health.Indicator {
	return &health.Indicator{Name: "redis:" + name, Probe: func(ctx context.Context) error {
		return HealthCheck(ctx, c)
	}}
}
