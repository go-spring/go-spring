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

package StarterGormMySql

import (
	"net/url"
	"strings"
	"time"
)

// Config holds the configuration parameters for a MySQL connection.
type Config struct {
	User         string        `value:"${user}"`           // Database username
	Password     string        `value:"${password}"`       // Database password
	Network      string        `value:"${net:=}"`          // Network type (tcp, unix), optional
	Addr         string        `value:"${addr}"`           // Database host:port or socket path
	DB           string        `value:"${db}"`             // Database name
	Timeout      time.Duration `value:"${timeout:=}"`      // Connection timeout
	ReadTimeout  time.Duration `value:"${readTimeout:=}"`  // Read timeout
	WriteTimeout time.Duration `value:"${writeTimeout:=}"` // Write timeout
	Charset      string        `value:"${charset:=}"`      // Character set, e.g., utf8mb4
	ParseTime    bool          `value:"${parseTime:=}"`    // Parse time values into time.Time
	Location     string        `value:"${loc:=}"`          // Timezone location, e.g., Asia/Shanghai
}

// DSN constructs the MySQL Data Source Name based on the configuration.
func (c Config) DSN() string {
	var sb strings.Builder
	sb.WriteString(c.User)
	sb.WriteString(":")
	sb.WriteString(c.Password)
	sb.WriteString("@")

	network := c.Network
	if network == "" {
		network = "tcp"
	}

	sb.WriteString(network)
	sb.WriteString("(")
	sb.WriteString(c.Addr)
	sb.WriteString(")")
	sb.WriteString("/")
	sb.WriteString(c.DB)
	sb.WriteString("?")

	if c.Charset != "" {
		sb.WriteString("charset=")
		sb.WriteString(c.Charset)
		sb.WriteString("&")
	}

	if c.ParseTime {
		sb.WriteString("parseTime=true&")
	}

	if c.Location != "" {
		sb.WriteString("loc=")
		sb.WriteString(url.QueryEscape(c.Location))
		sb.WriteString("&")
	}

	if c.Timeout != 0 {
		sb.WriteString("timeout=")
		sb.WriteString(c.Timeout.String())
		sb.WriteString("&")
	}

	if c.ReadTimeout != 0 {
		sb.WriteString("readTimeout=")
		sb.WriteString(c.ReadTimeout.String())
		sb.WriteString("&")
	}

	if c.WriteTimeout != 0 {
		sb.WriteString("writeTimeout=")
		sb.WriteString(c.WriteTimeout.String())
		sb.WriteString("&")
	}

	s := sb.String()
	return s[:len(s)-1]
}
