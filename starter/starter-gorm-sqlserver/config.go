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
	"strings"
)

// Config holds the configuration parameters for a SQL Server connection.
type Config struct {
	User     string `value:"${user}"`         // Database username
	Password string `value:"${password}"`     // Database password
	Host     string `value:"${host}"`         // Database host
	Port     string `value:"${port:=1433}"`   // Database port
	DB       string `value:"${db}"`           // Database name

	// ServiceName is the service discovery name. When set, Host/Port are
	// ignored for dialing and the connection reaches a live instance resolved
	// from the discovery backend.
	ServiceName string `value:"${service-name:=}"`
	// Discovery selects which registered discovery backend resolves ServiceName.
	// Only consulted when ServiceName is set; defaults to "default".
	Discovery string `value:"${discovery:=default}"`
}

// DSN constructs the SQL Server Data Source Name based on the configuration.
// Format: sqlserver://<user>:<password>@<host>:<port>?database=<db>
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
	return sb.String()
}
