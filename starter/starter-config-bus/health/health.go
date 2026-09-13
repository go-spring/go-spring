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

	"go-spring.org/cloud/actuator/health"
)

// errNotSubscribed is the probe failure. The bus is alive but no longer
// listening, which is the failure a connectivity check cannot see.
var errNotSubscribed = errors.New("config bus subscription is not active")

// NewBusHealth builds an indicator for a config bus. It is registered alongside
// the bus and exported as health.Indicator, so an application that also imports
// starter-actuator gets the refresh listener folded into /readiness with no
// extra wiring.
//
// healthy reports whether the subscription is still active — the question the
// bus alone can answer. It takes a plain function rather than the bus itself so
// this package stays free of a dependency on the starter package, whose own
// registration imports this one.
func NewBusHealth(name string, healthy func() bool) *health.Indicator {
	return &health.Indicator{Name: "config-bus:" + name, Probe: func(context.Context) error {
		if !healthy() {
			return errNotSubscribed
		}
		return nil
	}}
}
