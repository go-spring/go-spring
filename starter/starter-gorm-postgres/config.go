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

package StarterGormPostgres

import (
	"strconv"
	"strings"
	"time"
)

// Config holds the configuration parameters for a PostgreSQL connection.
type Config struct {
	Host           string        `value:"${host:=}"`           // Database host (required unless ServiceName is set)
	Port           string        `value:"${port:=5432}"`       // Database port
	User           string        `value:"${user}"`             // Database username
	Password       string        `value:"${password}"`         // Database password
	DB             string        `value:"${db}"`               // Database name
	SSLMode        string        `value:"${sslmode:=disable}"` // SSL mode, e.g., disable, require, verify-full
	TimeZone       string        `value:"${timezone:=}"`       // Timezone, e.g., Asia/Shanghai
	ConnectTimeout time.Duration `value:"${connectTimeout:=}"` // Connection timeout

	// SSL certificate material. PostgreSQL negotiates TLS through SSLMode; these
	// paths supply the CA / client certificate / key when a verifying mode or
	// client-cert auth is used.
	SSLRootCert string `value:"${sslrootcert:=}"` // Path to CA certificate (PEM)
	SSLCert     string `value:"${sslcert:=}"`     // Path to client certificate (PEM)
	SSLKey      string `value:"${sslkey:=}"`      // Path to client private key (PEM)

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

	// ServiceName is the service discovery name. When set, Host/Port are ignored
	// and the connection dials a live instance resolved from the discovery backend.
	ServiceName string `value:"${service-name:=}"`
	// Discovery selects which registered discovery backend resolves ServiceName.
	// Only consulted when ServiceName is set; defaults to "default".
	Discovery string `value:"${discovery:=default}"`
}

// DSN constructs the PostgreSQL Data Source Name based on the configuration.
// PostgreSQL uses a space-separated key=value DSN, e.g.:
//
//	host=127.0.0.1 port=5432 user=postgres password=xxx dbname=test sslmode=disable
func (c Config) DSN() string {
	var sb strings.Builder
	sb.WriteString("host=")
	sb.WriteString(c.Host)
	sb.WriteString(" port=")
	sb.WriteString(c.Port)
	sb.WriteString(" user=")
	sb.WriteString(c.User)
	sb.WriteString(" password=")
	sb.WriteString(c.Password)
	sb.WriteString(" dbname=")
	sb.WriteString(c.DB)
	sb.WriteString(" sslmode=")
	sb.WriteString(c.SSLMode)

	if c.SSLRootCert != "" {
		sb.WriteString(" sslrootcert=")
		sb.WriteString(c.SSLRootCert)
	}
	if c.SSLCert != "" {
		sb.WriteString(" sslcert=")
		sb.WriteString(c.SSLCert)
	}
	if c.SSLKey != "" {
		sb.WriteString(" sslkey=")
		sb.WriteString(c.SSLKey)
	}

	if c.TimeZone != "" {
		sb.WriteString(" TimeZone=")
		sb.WriteString(c.TimeZone)
	}

	if c.ConnectTimeout != 0 {
		sb.WriteString(" connect_timeout=")
		sb.WriteString(strconv.Itoa(int(c.ConnectTimeout.Seconds())))
	}

	return sb.String()
}
