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

package StarterGormSqlserver

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Config holds the configuration parameters for a SQL Server connection.
type Config struct {
	User     string `value:"${user}"`       // Database username
	Password string `value:"${password}"`   // Database password
	Host     string `value:"${host:=}"`     // Database host (required unless ServiceName is set)
	Port     string `value:"${port:=1433}"` // Database port
	DB       string `value:"${db}"`         // Database name

	// Connect/dial timeouts. SQL Server has no DSN-level read/write timeout;
	// per-operation deadlines are driven through context. A zero value leaves
	// the driver default in place.
	DialTimeout    time.Duration `value:"${dialTimeout:=}"`    // TCP dial timeout
	ConnectTimeout time.Duration `value:"${connectTimeout:=}"` // Login/connection timeout

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

	// TLS configuration. TLSEnabled maps to the driver "encrypt=true" mode;
	// TLSSkipVerify maps to "TrustServerCertificate=true"; TLSCA supplies a PEM
	// server certificate/CA path via "certificate".
	TLSEnabled    bool   `value:"${tls-enabled:=false}"`     // Enable TLS encryption
	TLSSkipVerify bool   `value:"${tls-skip-verify:=false}"` // Trust the server certificate without verification
	TLSCA         string `value:"${tls-ca:=}"`               // Path to server certificate / CA (PEM)

	// ServiceName is the service discovery name. When set, Host/Port are
	// ignored for dialing and the connection reaches a live instance resolved
	// from the discovery backend.
	ServiceName string `value:"${service-name:=}"`
	// Discovery selects which registered discovery backend resolves ServiceName.
	// Only consulted when ServiceName is set; defaults to "default".
	Discovery string `value:"${discovery:=default}"`
}

// DSN constructs the SQL Server Data Source Name based on the configuration.
// Format: sqlserver://<user>:<password>@<host>:<port>?database=<db>&...
func (c Config) DSN() string {
	var sb strings.Builder
	sb.WriteString("sqlserver://")
	sb.WriteString(url.QueryEscape(c.User))
	sb.WriteString(":")
	sb.WriteString(url.QueryEscape(c.Password))
	sb.WriteString("@")
	sb.WriteString(c.Host)
	sb.WriteString(":")
	sb.WriteString(c.Port)
	sb.WriteString("?database=")
	sb.WriteString(url.QueryEscape(c.DB))

	if c.DialTimeout != 0 {
		sb.WriteString("&dial+timeout=")
		sb.WriteString(strconv.Itoa(int(c.DialTimeout.Seconds())))
	}
	if c.ConnectTimeout != 0 {
		sb.WriteString("&connection+timeout=")
		sb.WriteString(strconv.Itoa(int(c.ConnectTimeout.Seconds())))
	}
	if c.TLSEnabled {
		sb.WriteString("&encrypt=true")
		if c.TLSSkipVerify {
			sb.WriteString("&TrustServerCertificate=true")
		}
		if c.TLSCA != "" {
			sb.WriteString("&certificate=")
			sb.WriteString(url.QueryEscape(c.TLSCA))
		}
	}
	return sb.String()
}
