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

package StarterMilvus

import (
	"context"

	"go-spring.org/cloud/actuator/health"
)

// HealthCheck is the module's single connectivity probe: it lists collections —
// one round trip that verifies reachability and auth. It goes straight to the
// raw client on purpose — a readiness check must reflect the backend, not the
// rate limiter, and must not feed the guard's statistics — so it calls the
// embedded SDK client's ListCollections directly. The startup probe in
// [newClient] and the actuator probe in [NewClientHealth] both delegate here,
// so there is exactly one implementation.
func HealthCheck(ctx context.Context, c *Client) error {
	_, err := c.Client.ListCollections(ctx)
	return err
}

// NewClientHealth builds an indicator for a Milvus client. The probe only calls
// [HealthCheck], so it shares the module's one connectivity check.
func NewClientHealth(name string, c *Client) *health.Indicator {
	return &health.Indicator{Name: "milvus:" + name, Probe: func(ctx context.Context) error {
		return HealthCheck(ctx, c)
	}}
}
