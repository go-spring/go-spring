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
	"go-spring.org/stdlib/health"
	"go-spring.org/stdlib/starter"
)

// newClientHealth builds an indicator for a single/sentinel client. It is
// registered once per configured instance and exported as health.Indicator, so
// an application that also imports starter-actuator gets Redis readiness folded
// into /readiness with no extra wiring.
func newClientHealth(name string, client *redis.Client) health.Indicator {
	return starter.NewIndicator("redis:"+name, func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	})
}

// newClusterHealth builds an indicator for a cluster client.
func newClusterHealth(name string, client *redis.ClusterClient) health.Indicator {
	return starter.NewIndicator("redis:"+name, func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	})
}
