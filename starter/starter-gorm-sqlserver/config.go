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

	gormcore "go-spring.org/starter-gorm"
)

// Config holds the configuration parameters for a SQL Server connection. The
// shared pool/discovery/observe settings come from the embedded gormcore.Common;
// the fields below are the SQL Server-specific connection parameters.
type Config struct {
	gormcore.Common

	User     string `value:"${user}"`       // Database username
	Password string `value:"${password}"`   // Database password
	Host     string `value:"${host:=}"`     // Database host (required unless ServiceName is set)
	Port     string `value:"${port:=1433}"` // Database port
	DB       string `value:"${db}"`         // Database name

	// Connect/dial timeouts. SQL Server has no DSN-level read/write timeout;
	// per-operation deadlines are driven through context. A zero value leaves
	// the driver default in place. The DSN parameters take whole seconds, so
	// sub-second durations round up to 1s rather than truncating to 0 (= no
	// timeout).
	DialTimeout    time.Duration `value:"${dialTimeout:=}"`    // TCP dial timeout
	ConnectTimeout time.Duration `value:"${connectTimeout:=}"` // Login/connection timeout

	// TLS binds the tls.* keys the SQL Server DSN can actually express:
	// tls.enabled → "encrypt=true", tls.insecure-skip-verify →
	// "TrustServerCertificate=true", tls.ca-file → "certificate",
	// tls.server-name → "hostNameInCertificate". The wider tlsconf.TLSConfig
	// block is deliberately not used: its client-cert (cert-file/key-file) keys
	// have no DSN slot here, so binding them would advertise dead
	// configuration. mTLS therefore needs a custom connector.
	TLS TLSConfig `value:"${tls}"`
}

// TLSConfig is the SQL Server subset of the shared cloud/tlsconf block: only
// the keys that map onto DSN parameters. mTLS (client certificates) is not
// expressible through the sqlserver DSN; use a custom connector if you need it.
type TLSConfig struct {
	// Enabled turns on encryption ("encrypt=true").
	Enabled bool `value:"${enabled:=false}"`

	// InsecureSkipVerify maps to "TrustServerCertificate=true": trust the
	// server's certificate without validating it against a CA. Dev/testing only.
	InsecureSkipVerify bool `value:"${insecure-skip-verify:=false}"`

	// CAFile is a PEM server certificate / CA path mapped to the DSN
	// "certificate" parameter.
	CAFile string `value:"${ca-file:=}"`

	// ServerName overrides the name checked against the server certificate,
	// mapped to the DSN "hostNameInCertificate" parameter. Set it when dialing
	// by IP or through a discovery label, where the address no longer matches
	// the name in the certificate.
	ServerName string `value:"${server-name:=}"`
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
		sb.WriteString(strconv.Itoa(ceilSeconds(c.DialTimeout)))
	}
	if c.ConnectTimeout != 0 {
		sb.WriteString("&connection+timeout=")
		sb.WriteString(strconv.Itoa(ceilSeconds(c.ConnectTimeout)))
	}
	if c.TLS.Enabled {
		sb.WriteString("&encrypt=true")
		if c.TLS.InsecureSkipVerify {
			sb.WriteString("&TrustServerCertificate=true")
		}
		if c.TLS.CAFile != "" {
			sb.WriteString("&certificate=")
			sb.WriteString(url.QueryEscape(c.TLS.CAFile))
		}
		if c.TLS.ServerName != "" {
			sb.WriteString("&hostNameInCertificate=")
			sb.WriteString(url.QueryEscape(c.TLS.ServerName))
		}
	}
	return sb.String()
}

// ceilSeconds rounds a positive duration up to whole seconds. The DSN timeout
// parameters accept integers only; truncating would turn a sub-second timeout
// into 0, which the driver reads as "no timeout".
func ceilSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int((d + time.Second - 1) / time.Second)
}
