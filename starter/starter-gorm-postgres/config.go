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

	"go-spring.org/cloud/tlsconf"
	"go-spring.org/starter-gorm"
)

// Config holds the configuration parameters for a PostgreSQL connection. The
// shared pool/discovery/observe settings come from the embedded gormcore.Common;
// the fields below are the PostgreSQL-specific connection parameters.
type Config struct {
	gormcore.Common

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

	// TLS is the shared tlsconf.TLSConfig block (nested keys: tls.enabled,
	// tls.cert-file, tls.key-file, tls.ca-file, tls.server-name,
	// tls.insecure-skip-verify), symmetric with the mysql starter. When enabled,
	// the built *tls.Config is injected into the pgx connection config; sslmode
	// still decides whether TLS is negotiated at all, so tls.enabled together
	// with sslmode=disable is rejected at startup instead of silently dialing
	// plaintext.
	TLS tlsconf.TLSConfig `value:"${tls}"`
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
		sb.WriteString(strconv.Itoa(ceilSeconds(c.ConnectTimeout)))
	}

	return sb.String()
}

// ceilSeconds rounds a positive duration up to whole seconds. The libpq-style
// connect_timeout accepts integers only; truncating would turn a sub-second
// timeout into 0, which pgx reads as "no timeout".
func ceilSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int((d + time.Second - 1) / time.Second)
}
