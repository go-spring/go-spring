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

package StarterInfluxdb

import (
	"context"
	"fmt"

	"github.com/influxdata/influxdb-client-go/v2/domain"
	"go-spring.org/cloud/actuator/health"
)

// NewClientHealth builds an indicator for an InfluxDB client. It is
// registered once per configured instance and exported as health.Indicator,
// so an application that also imports starter-actuator gets InfluxDB
// readiness folded into /readiness with no extra wiring.
//
// The probe delegates to [HealthCheck], the single liveness implementation:
// there is one place a readiness check is defined, so the indicator and an
// ad-hoc caller can never drift apart.
func NewClientHealth(name string, c *Client) *health.Indicator {
	return &health.Indicator{Name: "influxdb:" + name, Probe: func(ctx context.Context) error {
		return HealthCheck(ctx, c)
	}}
}

// healthError maps a domain.HealthCheck onto an error: nil when the server
// reports pass, the reported message otherwise. It is the /health status
// mapping [HealthCheck] is built on; it stays unexported because HealthCheck
// is the only liveness entry this package exports.
func healthError(hc *domain.HealthCheck) error {
	if hc.Status == domain.HealthCheckStatusPass {
		return nil
	}
	msg := ""
	if hc.Message != nil {
		msg = *hc.Message
	}
	return fmt.Errorf("influxdb: health status %s: %s", hc.Status, msg)
}
