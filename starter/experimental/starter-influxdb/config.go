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
	observe "go-spring.org/cloud/observe"
)

// Config defines the InfluxDB 2.x client configuration.
type Config struct {
	// ServerURL is the InfluxDB base URL, e.g., "http://127.0.0.1:8086".
	ServerURL string `value:"${server-url}" expr:"$ != ''"`

	// AuthToken is the API token used for authentication.
	AuthToken string `value:"${auth-token}" expr:"$ != ''"`

	// Org is the default organization for writes and queries performed
	// through the wrapper helpers.
	Org string `value:"${org:=}"`

	// Bucket is the default destination bucket for writes performed through
	// the wrapper helpers.
	Bucket string `value:"${bucket:=}"`

	// Observability configures the per-request span, metric and access log
	// emitted by the observe transport (off/brief/detailed). Defaults to
	// "brief". Instance keys (spring.influxdb.<name>.observability.*) override
	// the top-level observability.* keys — see Client.resolveObservability.
	Observability observe.ObserveConfig `value:"${observability:=}"`

	// Driver specifies which InfluxDB driver to use, defaults to
	// DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}
