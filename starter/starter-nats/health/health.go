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
	"errors"

	"github.com/nats-io/nats.go"
	"go-spring.org/cloud/actuator/health"
)

// errNotConnected is the probe failure. It is a sentinel rather than a wrapped
// client error because the connection state, not a failed operation, is what
// the probe reports.
var errNotConnected = errors.New("nats connection is not established")

// NewConnHealth builds an indicator for a NATS connection. It is registered once
// per configured instance and exported as health.Indicator, so an application
// that also imports starter-actuator gets nats connectivity folded into
// /readiness with no extra wiring.
//
// The probe reads the live state of the auto-reconnecting client rather than the
// outcome of the initial dial, so a connection that dropped after startup reports
// unhealthy until it reconnects. It takes the raw *nats.Conn (like the redis
// indicators take their client) so this package stays free of a dependency on
// the starter package itself.
func NewConnHealth(name string, nc *nats.Conn) *health.Indicator {
	return &health.Indicator{Name: "nats:" + name, Probe: func(context.Context) error {
		if nc == nil || !nc.IsConnected() {
			return errNotConnected
		}
		return nil
	}}
}
