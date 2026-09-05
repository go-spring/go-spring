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

package StarterTdengine

import (
	"time"
)

// Config defines the TDengine client configuration.
//
// The starter speaks the websocket wire protocol through the official
// driver-go/v3 taosWS driver — pure Go, no client library install, no CGO.
type Config struct {
	// DSN is the TDengine websocket DSN in the driver's unified format:
	// [user[:password]@]ws(host:port)/[dbname][?params],
	// e.g., "root:taosdata@ws(127.0.0.1:6041)/power".
	DSN string `value:"${dsn}" expr:"$ != ''"`

	// MaxOpenConns limits the connection pool size (0 = unlimited).
	MaxOpenConns int `value:"${max-open-conns:=8}"`

	// MaxIdleConns limits the idle pool size.
	MaxIdleConns int `value:"${max-idle-conns:=2}"`

	// ConnMaxLifetime retires connections after this age (0 = never).
	ConnMaxLifetime time.Duration `value:"${conn-max-lifetime:=0s}"`

	// Driver specifies which TDengine driver to use, defaults to
	// DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}
