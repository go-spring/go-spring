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

package StarterActuator

import "time"

// Config is the actuator's configuration under ${spring.actuator}. It carries
// only configured values; the server that consumes it lives in server.go.
//
// Enabled follows the starter convention: every server-shaped starter's config
// starts with an enabled switch defaulting to on (spring.actuator.enabled),
// paired with the addr key that must still be set explicitly — the switch
// opts OUT, the address opts IN.
type Config struct {
	// Enabled gates the starter (spring.actuator.enabled, default true). The
	// starter still requires spring.actuator.addr to be set; this switch is
	// how a deployment that configures the address turns the management port
	// off without deleting config.
	Enabled bool `value:"${enabled:=true}"`

	// Address is the management listen address. There is no default; setting
	// this key is what activates the starter. Documented layout: main HTTP
	// server (:9090), actuator (:9370, all interfaces so in-cluster probes can
	// reach it), pprof (127.0.0.1:9981).
	Address string `value:"${addr}"`

	// CheckTimeout bounds a single probe check across all indicators so one
	// slow dependency cannot stall the probe past a typical kubelet probe
	// timeout (spring.actuator.check-timeout, default 3s).
	CheckTimeout time.Duration `value:"${check-timeout:=3s}" expr:"$ > 0"`

	// ReadHeaderTimeout is the deadline for reading request headers, the
	// slowloris guard (spring.actuator.read-header-timeout, default 5s). Zero
	// disables it.
	ReadHeaderTimeout time.Duration `value:"${read-header-timeout:=5s}" expr:"$ >= 0"`

	// ReadTimeout, WriteTimeout and IdleTimeout mirror http.Server's fields
	// (spring.actuator.read-timeout / write-timeout / idle-timeout, default 0 =
	// no limit). A management port serves short probe/metrics requests, so the
	// defaults leave these off; set WriteTimeout if the port is reachable from
	// untrusted networks.
	ReadTimeout  time.Duration `value:"${read-timeout:=0}" expr:"$ >= 0"`
	WriteTimeout time.Duration `value:"${write-timeout:=0}" expr:"$ >= 0"`
	IdleTimeout  time.Duration `value:"${idle-timeout:=0}" expr:"$ >= 0"`

	// Token, when set, requires an "Authorization: Bearer <token>" header on
	// every request to the management port (spring.actuator.token). Takes
	// precedence over Username/Password.
	Token string `value:"${token:=}"`

	// Username and Password, when both set, require HTTP Basic authentication
	// on the management port (spring.actuator.username /
	// spring.actuator.password).
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`
}
