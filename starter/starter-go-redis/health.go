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

package StarterGoRedis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// redisHealth adapts a Redis client into a health.Indicator. It is registered
// once per configured instance and exported as health.Indicator, so an
// application that also imports starter-actuator gets Redis readiness folded
// into /readiness with no extra wiring. When the actuator is absent the bean is
// simply never collected.
type redisHealth struct {
	name   string
	client redis.UniversalClient
}

// HealthName identifies the instance under its configured name, e.g.
// "redis:cache".
func (h *redisHealth) HealthName() string { return "redis:" + h.name }

// CheckHealth reports the connection as healthy when PING succeeds within the
// caller's context deadline.
func (h *redisHealth) CheckHealth(ctx context.Context) error {
	return h.client.Ping(ctx).Err()
}

// newClientHealth builds an indicator for a single/sentinel client.
func newClientHealth(name string, client *redis.Client) *redisHealth {
	return &redisHealth{name: name, client: client}
}

// newClusterHealth builds an indicator for a cluster client.
func newClusterHealth(name string, client *redis.ClusterClient) *redisHealth {
	return &redisHealth{name: name, client: client}
}
