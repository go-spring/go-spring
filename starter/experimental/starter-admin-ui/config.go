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

package StarterAdminUI

import "time"

// Config carries the settings bound from the "spring.admin-ui.*" property tree.
// The tag on the Server.Config field ("${spring.admin-ui}") sets the prefix;
// the tags on the fields below are keyed relative to that prefix.
//
// It is a value struct (not a bean): the container binds it, the Server holds
// it, and no other consumer needs to inject it.
type Config struct {
	// Addr is the listen address for the Admin UI HTTP server. It defaults to
	// :9280 — distinct from the application's main HTTP server (:9090), the
	// actuator management port (:9370), and pprof (127.0.0.1:9981) — so all
	// four can coexist in a single process during local development.
	Addr string `value:"${addr:=:9280}"`

	// Instances is the list of actuator base URLs the UI polls, e.g.
	// "http://10.0.0.1:9370". Empty is intentionally allowed: an operator may
	// configure instances later without redeploying, and the UI degrades to a
	// "no instances configured" state rather than failing to start.
	Instances []string `value:"${instances:=}"`

	// Interval is how often the background poller refreshes the snapshot. A
	// short default keeps the dashboard usefully live while staying cheap: with
	// N instances and 4 endpoints each the load is 4N/Interval requests/sec.
	Interval time.Duration `value:"${interval:=10s}"`

	// Timeout bounds a single HTTP call to one endpoint on one instance. Kept
	// small so a slow or wedged instance cannot stall the whole sweep beyond
	// roughly (4 endpoints * Timeout) per instance.
	Timeout time.Duration `value:"${timeout:=3s}"`

	// Title is rendered in the page <title> and header. Configurable so the
	// same binary shipped to multiple environments can label the dashboard
	// with the environment name ("prod cluster", "staging", ...).
	Title string `value:"${title:=Go-Spring Admin}"`
}
