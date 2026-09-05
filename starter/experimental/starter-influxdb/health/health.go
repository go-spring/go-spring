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
	"fmt"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"go-spring.org/cloud/actuator/health"
)

// NewClientHealth builds an indicator for an InfluxDB client. It is
// registered once per configured instance and exported as health.Indicator,
// so an application that also imports starter-actuator gets InfluxDB
// readiness folded into /readiness with no extra wiring.
//
// The probe calls /health: it verifies reachability, authentication setup and
// server status in one round trip.
func NewClientHealth(name string, client influxdb2.Client) *health.Indicator {
	return &health.Indicator{Name: "influxdb:" + name, Probe: func(ctx context.Context) error {
		hc, err := client.Health(ctx)
		if err != nil {
			return err
		}
		return HealthError(hc)
	}}
}

// HealthError maps a domain.HealthCheck onto an error: nil when the server
// reports pass, the reported message otherwise. It is shared by the starter's
// fail-fast probe and the health indicator.
func HealthError(hc *domain.HealthCheck) error {
	if hc.Status == domain.HealthCheckStatusPass {
		return nil
	}
	msg := ""
	if hc.Message != nil {
		msg = *hc.Message
	}
	return fmt.Errorf("influxdb: health status %s: %s", hc.Status, msg)
}
