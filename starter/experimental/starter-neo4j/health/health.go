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

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud/actuator/health"
)

// NewDriverHealth builds an indicator for a Neo4j driver. It is registered once
// per configured instance and exported as health.Indicator, so an application
// that also imports starter-actuator gets Neo4j readiness folded into
// /readiness with no extra wiring.
func NewDriverHealth(name string, client neo4j.DriverWithContext) *health.Indicator {
	return &health.Indicator{Name: "neo4j:" + name, Probe: func(ctx context.Context) error {
		return client.VerifyConnectivity(ctx)
	}}
}
