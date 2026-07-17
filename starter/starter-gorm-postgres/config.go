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
	Host           string        `value:"${host}"`               // Database host
	Port           string        `value:"${port:=5432}"`         // Database port
	User           string        `value:"${user}"`               // Database username
	Password       string        `value:"${password}"`           // Database password
	DB             string        `value:"${db}"`                 // Database name
	SSLMode        string        `value:"${sslmode:=disable}"`   // SSL mode, e.g., disable, require, verify-full
	TimeZone       string        `value:"${timezone:=}"`         // Timezone, e.g., Asia/Shanghai
	ConnectTimeout time.Duration `value:"${connectTimeout:=}"`   // Connection timeout
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
