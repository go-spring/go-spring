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
)

// NewClientHealth builds an indicator for a Milvus client. The probe lists
// collections — one round trip that verifies reachability and auth.
func NewClientHealth(name string, c interface {
	Health(ctx context.Context) error
}) *health.Indicator {
	return &health.Indicator{Name: "milvus:" + name, Probe: func(ctx context.Context) error {
		return c.Health(ctx)
	}}
}
