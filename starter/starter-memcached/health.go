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
// memcached readiness folded into /readiness with no extra wiring.

package StarterMemcached

import (
	"context"

	"go-spring.org/cloud/actuator/health"
)

// HealthCheck probes a memcached client with the cheapest read-only round trip:
// a PING to every server in the pool. It is the module's single health
// implementation — the Actuator probe ([NewClientHealth]) and the startup
// probe in the constructor both delegate to it.
//
// It goes straight to the raw client on purpose: a connectivity check must
// reflect the backend, not the rate limiter, and must not feed the operation
// metrics or the breaker's statistics. gomemcache's Ping carries no context, so
// ctx cannot bound the probe; the client's own dial/read timeouts do.
func HealthCheck(ctx context.Context, c *Client) error {
	return c.Client.Ping()
}

// NewClientHealth builds an indicator for a memcached client. It is registered
// once per configured instance and exported as health.Indicator, so an
// application that also imports starter-actuator gets memcached readiness
// folded into /readiness with no extra wiring. The probe delegates to
// [HealthCheck], the module's single health implementation.
func NewClientHealth(name string, c *Client) *health.Indicator {
	return &health.Indicator{Name: "memcache:" + name, Probe: func(ctx context.Context) error {
		return HealthCheck(ctx, c)
	}}
}
