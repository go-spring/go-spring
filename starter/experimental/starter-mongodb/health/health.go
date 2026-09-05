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

package health2

import (
	"context"

	"go-spring.org/cloud/actuator/health"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// NewClientHealth builds an indicator for a MongoDB client. It is registered
// once per configured instance and exported as health.Indicator, so an
// application that also imports starter-actuator gets MongoDB readiness folded
// into /readiness with no extra wiring.
func NewClientHealth(name string, client *mongo.Client) *health.Indicator {
	return &health.Indicator{Name: "mongo:" + name, Probe: func(ctx context.Context) error {
		return client.Ping(ctx, nil)
	}}
}
