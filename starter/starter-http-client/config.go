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

package StarterHTTPClient

import (
	"fmt"
	"net/http"
	"time"

	"go-spring.org/spring/httpx"
	"go-spring.org/spring/resilience"
)

// Config binds one declarative-HTTP-client instance under
// "${spring.http-client}". Each entry yields a named *http.Client whose
// transport is assembled by stdlib/httpx: service discovery + load balancing
// when ServiceName is set, or a fixed address otherwise, optionally protected
// by resilience and always traced through the OTel globals. A generated client
// (gs-http-gen) is wired by injecting this *http.Client into its HTTPClient
// field, so switching a call between a direct address and a discovered service
// is a pure-config change.
type Config struct {
	// Addr is the direct "host:port" to call. Used when ServiceName is empty;
	// mutually exclusive with it.
	Addr string `value:"${addr:=}"`

	// ServiceName routes through service discovery and load balancing instead of
	// a fixed address. Mutually exclusive with Addr.
	ServiceName string `value:"${service-name:=}"`

	// Discovery names the registered discovery backend (from stdlib/discovery)
	// that resolves ServiceName. Required when ServiceName is set.
	Discovery string `value:"${discovery:=}"`

	// Balancer names the load-balancing strategy: round_robin (default),
	// least_conn, consistent_hash, weighted, or zone_aware.
	Balancer string `value:"${balancer:=round_robin}"`

	// EjectThreshold is the consecutive-failure count that ejects a failing
	// endpoint from the pool (outlier ejection). 0 disables ejection.
	EjectThreshold int `value:"${eject-threshold:=0}"`

	// EjectFor is how long an ejected endpoint stays out before a trial request.
	// Ignored when EjectThreshold is 0.
	EjectFor time.Duration `value:"${eject-for:=0}"`

	// Timeout bounds each request made by the client. 0 means no timeout.
	Timeout time.Duration `value:"${timeout:=0}"`

	// Resilience optionally protects outbound requests with rate limiting,
	// circuit breaking and retry. Disabled by default.
	Resilience ResilienceConfig `value:"${resilience:=}"`
}

// ResilienceConfig binds the backend-neutral resilience knobs exposed by
// stdlib/resilience. Driver selects which registered backend enforces them:
// "default" (bundled) or "sentinel" (blank-import starter-resilience). It
// mirrors the shape used by starter-oauth2-client so the two client families
// read the same in configuration.
type ResilienceConfig struct {
	// Enabled turns the resilience transport on. When false the client is
	// returned unwrapped.
	Enabled bool `value:"${enabled:=false}"`

	// Driver names the registered resilience backend to use.
	Driver string `value:"${driver:=default}"`

	// RateLimit caps sustained throughput in requests per second (0 disables).
	RateLimit float64 `value:"${rate-limit:=0}"`

	// Burst is the momentary allowance above RateLimit (0 = driver default).
	Burst int `value:"${burst:=0}"`

	// ErrorThreshold is the consecutive-failure count that trips the breaker
	// open (0 disables circuit breaking).
	ErrorThreshold int `value:"${error-threshold:=0}"`

	// OpenDuration is how long the breaker stays open before a trial request.
	OpenDuration time.Duration `value:"${open-duration:=0}"`

	// MaxRetries is the number of extra attempts after the first failure.
	MaxRetries int `value:"${max-retries:=0}"`

	// AttemptTimeout bounds each individual attempt (0 = no per-attempt bound).
	AttemptTimeout time.Duration `value:"${attempt-timeout:=0}"`
}

// validate enforces the addr-or-service-name fail-fast rule shared by client
// starters: exactly one addressing mode, and discovery is mandatory when
// routing by service name. go-spring's expr: tag validates one field at a time,
// so this cross-field rule lives here rather than in a tag.
func (c Config) validate() error {
	switch {
	case c.Addr == "" && c.ServiceName == "":
		return fmt.Errorf("http-client: one of addr or service-name is required")
	case c.Addr != "" && c.ServiceName != "":
		return fmt.Errorf("http-client: addr and service-name are mutually exclusive")
	case c.ServiceName != "" && c.Discovery == "":
		return fmt.Errorf("http-client: discovery is required when service-name is set")
	}
	return nil
}

// toTransportConfig maps the bound Config onto the stdlib/httpx assembler input,
// applying base as the underlying (trace-instrumented) transport.
func (c Config) toTransportConfig(base http.RoundTripper) httpx.Config {
	cfg := httpx.Config{
		ServiceName:    c.ServiceName,
		Addr:           c.Addr,
		Discovery:      c.Discovery,
		Balancer:       c.Balancer,
		EjectThreshold: c.EjectThreshold,
		EjectFor:       c.EjectFor,
		Base:           base,
	}
	if c.Resilience.Enabled {
		cfg.ResilienceDriver = c.Resilience.Driver
		cfg.ResiliencePolicy = resilience.Policy{
			RateLimit:      c.Resilience.RateLimit,
			Burst:          c.Resilience.Burst,
			ErrorThreshold: c.Resilience.ErrorThreshold,
			OpenDuration:   c.Resilience.OpenDuration,
			MaxRetries:     c.Resilience.MaxRetries,
			Timeout:        c.Resilience.AttemptTimeout,
		}
	}
	return cfg
}
