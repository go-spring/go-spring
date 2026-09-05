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

	"github.com/gocql/gocql"
	"go-spring.org/cloud/actuator/health"
)

// NewClientHealth builds an indicator for a Cassandra session. It is
// registered once per configured instance and exported as health.Indicator,
// so an application that also imports starter-actuator gets Cassandra
// readiness folded into /readiness with no extra wiring.
//
// The probe scans system.local on the contact point: one round trip that
// verifies protocol, auth and cluster state.
func NewClientHealth(name string, session *gocql.Session) *health.Indicator {
	return &health.Indicator{Name: "cassandra:" + name, Probe: func(ctx context.Context) error {
		var release string
		return session.Query("SELECT release_version FROM system.local").WithContext(ctx).Scan(&release)
	}}
}
