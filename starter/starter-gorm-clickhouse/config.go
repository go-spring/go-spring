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

	"go-spring.org/cloud/tlsconf"
	gormcore "go-spring.org/starter-gorm"
)

// Config holds the configuration parameters for a ClickHouse connection. The
// shared pool/discovery/observe settings come from the embedded gormcore.Common;
// the fields below are the ClickHouse-specific connection parameters.
type Config struct {
	gormcore.Common

	User        string        `value:"${user:=default}"` // Database username, default "default"
	Password    string        `value:"${password:=}"`    // Database password
	Addr        string        `value:"${addr:=}"`        // Database host:port (native protocol, typically 9000; required unless ServiceName is set)
	DB          string        `value:"${db:=default}"`   // Database name, default "default"
	DialTimeout time.Duration `value:"${dialTimeout:=}"` // Connection dial timeout, optional
	ReadTimeout time.Duration `value:"${readTimeout:=}"` // Read timeout, optional

	// TLS configuration. When TLS.Enabled is set, the native ClickHouse driver
	// negotiates a secure connection using the *tls.Config produced by the
	// shared tlsconf.TLSConfig builder. Keys are nested
	// (spring.gorm.clickhouse.instances.<name>.tls.enabled, ...tls.cert-file, ...).
	TLS tlsconf.TLSConfig `value:"${tls}"`
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
