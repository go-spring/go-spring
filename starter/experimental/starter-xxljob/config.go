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

package StarterXxljob

import "time"

// Config defines one xxl-job executor instance. It speaks the xxl-job
// executor protocol to an admin (registry/heartbeat/run/kill) over plain
// HTTP, hand-rolled (no third-party SDK) — see DESIGN.
type Config struct {
	// AppName is the executor app name as registered with the admin. Required.
	AppName string `value:"${app-name}" expr:"$ != ''"`

	// AdminAddresses is the xxl-job admin base URL list, e.g.
	// "http://127.0.0.1:8080/xxl-job-admin". Required; the registry/heartbeat
	// calls are load-balanced across entries.
	AdminAddresses []string `value:"${admin-addresses}" expr:"len($) > 0"`

	// Port is the TCP port the executor's callback server listens on. Required
	// — an executor server must be reachable by the admin to receive trigger
	// callbacks, so the port is an explicit operator decision (see
	// starter-server-port-must-be-configured).
	Port int `value:"${port}" expr:"$ > 0"`

	// AccessToken authenticates executor<->admin calls when the admin has one
	// set. Empty means no token (matches an admin without a token).
	AccessToken string `value:"${access-token:=}"`

	// RegistryInterval is how often the executor re-registers / heartbeats to
	// the admin.
	RegistryInterval time.Duration `value:"${registry-interval:=10s}"`

	// LogDir is the directory the executor SERVES per-task log files from via
	// the /log callback (and creates if absent). The executor itself never
	// writes task log files — the application's TaskFunc owns writing
	// <log-dir>/<logId>.log; this starter only owns the reading side.
	LogDir string `value:"${log-dir:=./logs}"`
}
