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

package StarterGormClickhouse

import (
	"net/url"
	"strings"
	"time"

	"go-spring.org/stdlib/starter"
)

// Config holds the configuration parameters for a ClickHouse connection.
type Config struct {
	User        string        `value:"${user:=default}"` // Database username, default "default"
	Password    string        `value:"${password:=}"`    // Database password
	Addr        string        `value:"${addr:=}"`        // Database host:port (native protocol, typically 9000; required unless ServiceName is set)
	DB          string        `value:"${db:=default}"`   // Database name, default "default"
	DialTimeout time.Duration `value:"${dialTimeout:=}"` // Connection dial timeout, optional
	ReadTimeout time.Duration `value:"${readTimeout:=}"` // Read timeout, optional

	// Connection pool tuning. A zero value leaves the database/sql default in
	// place (see sql.DB.SetMaxOpenConns and friends).
	MaxOpenConns    int           `value:"${max-open-conns:=0}"`     // Max open connections (0 = unlimited)
	MaxIdleConns    int           `value:"${max-idle-conns:=0}"`     // Max idle connections (0 = default 2)
	ConnMaxLifetime time.Duration `value:"${conn-max-lifetime:=0}"`  // Max lifetime of a connection (0 = unlimited)
	ConnMaxIdleTime time.Duration `value:"${conn-max-idle-time:=0}"` // Max idle time of a connection (0 = unlimited)

	// PingTimeout bounds the startup connectivity check. The client fails fast
	// during creation if the server cannot be reached within this window.
	PingTimeout time.Duration `value:"${ping-timeout:=5s}"`

	// SlowThreshold enables GORM slow-query logging when > 0: queries slower than
	// this are logged at warn level.
	SlowThreshold time.Duration `value:"${slow-threshold:=0}"`

	// TLS configuration. When TLS.Enabled is set, the native ClickHouse driver
	// negotiates a secure connection using the *tls.Config produced by the
	// shared stdlib/starter.TLSConfig builder. Keys are nested
	// (spring.gorm.clickhouse.<name>.tls.enabled, ...tls.cert-file, ...).
	TLS starter.TLSConfig `value:"${tls}"`

	// ServiceName is the service discovery name. When set, Addr is ignored and
	// the connection dials a live instance resolved from the discovery backend.
	ServiceName string `value:"${service-name:=}"`
	// Discovery selects which registered discovery backend resolves ServiceName.
	// Only consulted when ServiceName is set; defaults to "default".
	Discovery string `value:"${discovery:=default}"`
}

// DSN constructs the ClickHouse URL-style Data Source Name based on the configuration.
// Format: clickhouse://<user>:<password>@<addr>/<db>?dial_timeout=<dur>&read_timeout=<dur>
func (c Config) DSN() string {
	var sb strings.Builder
	sb.WriteString("clickhouse://")
	sb.WriteString(url.QueryEscape(c.User))
	sb.WriteString(":")
	sb.WriteString(url.QueryEscape(c.Password))
	sb.WriteString("@")
	sb.WriteString(c.Addr)
	sb.WriteString("/")
	sb.WriteString(c.DB)
	sb.WriteString("?")

	if c.DialTimeout != 0 {
		sb.WriteString("dial_timeout=")
		sb.WriteString(c.DialTimeout.String())
		sb.WriteString("&")
	}

	if c.ReadTimeout != 0 {
		sb.WriteString("read_timeout=")
		sb.WriteString(c.ReadTimeout.String())
		sb.WriteString("&")
	}

	s := sb.String()
	return s[:len(s)-1]
}
