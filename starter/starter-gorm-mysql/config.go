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

	"go-spring.org/spring/cloud/resilience"
	"go-spring.org/spring/cloud/tlsconf"
)

// Config holds the configuration parameters for a MySQL connection.
type Config struct {
	User         string        `value:"${user}"`           // Database username
	Password     string        `value:"${password}"`       // Database password
	Network      string        `value:"${net:=}"`          // Network type (tcp, unix), optional
	Addr         string        `value:"${addr:=}"`         // Database host:port or socket path (required unless ServiceName is set)
	DB           string        `value:"${db}"`             // Database name
	Timeout      time.Duration `value:"${timeout:=}"`      // Connection timeout
	ReadTimeout  time.Duration `value:"${readTimeout:=}"`  // Read timeout
	WriteTimeout time.Duration `value:"${writeTimeout:=}"` // Write timeout
	Charset      string        `value:"${charset:=}"`      // Character set, e.g., utf8mb4
	ParseTime    bool          `value:"${parseTime:=}"`    // Parse time values into time.Time
	Location     string        `value:"${loc:=}"`          // Timezone location, e.g., Asia/Shanghai

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

	// TLS configuration. When TLS.Enabled is set, the connection negotiates TLS
	// and a *tls.Config built from CA/cert/key/server-name/insecure-skip-verify
	// is registered with the mysql driver, then referenced in the DSN as
	// tls=<unique-name>. Uses the shared tlsconf.TLSConfig block, so keys
	// are nested (spring.gorm.mysql.<name>.tls.enabled, ...tls.cert-file, ...).
	TLS tlsconf.TLSConfig `value:"${tls}"`

	// ServiceName is the service discovery name. When set, Addr is ignored and
	// the connection dials a live instance resolved from the discovery backend.
	ServiceName string `value:"${service-name:=}"`
	// Discovery selects which registered discovery backend resolves ServiceName.
	// Only consulted when ServiceName is set; defaults to "default".
	Discovery string `value:"${discovery:=default}"`

	// Resilience optionally protects MySQL operations with rate limiting and
	// circuit breaking. It is disabled by default; when enabled a set of gorm
	// callbacks routes every DB op through the selected resilience driver.
	// For a DB, max-retries is best left at 0: retrying a write re-sends it, and
	// gorm/database-sql already retry at their layer.
	Resilience ResilienceConfig `value:"${resilience:=}"`

	// tlsParam carries the resolved MySQL DSN "tls" value (built-in mode name or
	// a registered custom config name). It is set internally, not bound from
	// configuration.
	tlsParam string
}

// ResilienceConfig binds the backend-neutral resilience knobs exposed by
// stdlib/resilience. Driver selects which registered backend enforces them:
// "default" (bundled, zero-dependency) or "sentinel" (recommended, enabled by
// blank-importing starter-resilience). Switching backends is a one-line config
// change — no code touches the callback seam.
type ResilienceConfig struct {
	// Enabled attaches the resilience callbacks. When false the client is unchanged.
	Enabled bool `value:"${enabled:=false}"`

	// Driver names the registered resilience backend to use.
	Driver string `value:"${driver:=default}"`

	// RateLimit caps sustained throughput in operations per second (0 disables).
	RateLimit float64 `value:"${rate-limit:=0}"`

	// Burst is the momentary allowance above RateLimit (0 = driver default).
	Burst int `value:"${burst:=0}"`

	// ErrorThreshold is the consecutive-failure count that trips the breaker
	// open (0 disables circuit breaking). gorm.ErrRecordNotFound never counts.
	ErrorThreshold int `value:"${error-threshold:=0}"`

	// OpenDuration is how long the breaker stays open before a trial operation.
	OpenDuration time.Duration `value:"${open-duration:=0}"`

	// MaxRetries is the number of extra attempts after a failure. Keep 0 for a
	// DB: retrying a write re-sends it, and gorm/database-sql already retry at
	// their layer.
	MaxRetries int `value:"${max-retries:=0}"`

	// AttemptTimeout bounds each individual attempt (0 = no per-attempt bound).
	AttemptTimeout time.Duration `value:"${attempt-timeout:=0}"`
}

// policy maps the bound config onto the backend-neutral resilience.Policy.
func (r ResilienceConfig) policy() resilience.Policy {
	return resilience.Policy{
		RateLimit:      r.RateLimit,
		Burst:          r.Burst,
		ErrorThreshold: r.ErrorThreshold,
		OpenDuration:   r.OpenDuration,
		MaxRetries:     r.MaxRetries,
		Timeout:        r.AttemptTimeout,
	}
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

	if c.tlsParam != "" {
		sb.WriteString("tls=")
		sb.WriteString(c.tlsParam)
		sb.WriteString("&")
	}

	s := sb.String()
	return s[:len(s)-1]
}
