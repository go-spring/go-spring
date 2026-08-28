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

	"go-spring.org/cloud/experimental/httpx"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
	"go-spring.org/cloud/observe/resilience"
	"go-spring.org/cloud/tlsconf"
)

// Config binds one declarative-HTTP-client entry under
// "${spring.http-client}". Each entry contributes a route whose transport is
// assembled by stdlib/httpx: service discovery + load balancing when
// ServiceName is set, or a fixed address otherwise, optionally protected by
// resilience and always traced through the OTel globals. All entries share one
// process-wide client (installed by replacing httpclt.DoRequest); a generated
// client routes by its Target (addr or service-name), so switching a call
// between a direct address and a discovered service is a pure-config change.
type Config struct {
	// Addr is the direct "host:port" to call. Used when ServiceName is empty;
	// mutually exclusive with it.
	Addr string `value:"${addr:=}"`

	// ServiceName routes through service discovery and load balancing instead of
	// a fixed address. It may also be set ALONGSIDE Addr: Addr then pins the
	// direct address while ServiceName remains the (stable) governance resource
	// label — so switching an entry between direct and discovery addressing does
	// not silently change the key govern.rules match on.
	ServiceName string `value:"${service-name:=}"`

	// Discovery names the registered discovery backend (from cloud/discovery)
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

	// Observability configures the resilience access log (off/brief/detailed)
	// emitted alongside the trace span + metrics that observe-resilience wraps
	// the executor with.
	Observability observe.ObserveConfig `value:"${observability:=}"`

	// TLS configures the certificate surface for https targets: a client key
	// pair (tls.cert-file/key-file), a CA bundle (tls.ca-file), the expected
	// peer name (tls.server-name) and an insecure escape hatch. Off by default;
	// when enabled it is wired into the transport's TLS config, so
	// WithScheme("https") gets verifiable TLS instead of system defaults.
	TLS tlsconf.TLSConfig `value:"${tls:=}"`
}

// Resilience and fault policy are NOT bound here: they live process-wide under
// govern.* (starter-governance). Keep govern retry counts at 0 unless requests
// are idempotent: the client may issue POSTs and other non-idempotent verbs,
// and a retry re-sends them.

// validate enforces the addr-or-service-name fail-fast rule shared by client
// starters: at least one of them must be set, Addr selects direct addressing
// (with ServiceName kept as a pure governance label), and discovery is
// mandatory when routing by service name alone. go-spring's expr: tag validates
// one field at a time, so this cross-field rule lives here rather than in a tag.
func (c Config) validate() error {
	switch {
	case c.Addr == "" && c.ServiceName == "":
		return fmt.Errorf("http-client: one of addr or service-name is required")
	case c.Addr == "" && c.Discovery == "":
		return fmt.Errorf("http-client: discovery is required when service-name is set without addr")
	}
	return nil
}

// toTransportConfig maps the bound Config onto the stdlib/httpx assembler input,
// applying base as the underlying (trace-instrumented) transport. exec is the
// resilience executor resolved via [resilience.ExecutorFor] (always non-nil — a
// transparent no-op when governance is off); it is handed to httpx directly, and
// WrapExec layers fault (innermost) + observe-resilience on top of it.
func (c Config) toTransportConfig(base http.RoundTripper, exec resilience.Executor) httpx.Config {
	serviceName := c.ServiceName
	if c.Addr != "" {
		// Direct addressing: service-name (when set) is a pure governance label,
		// NOT a discovery target — blank it so httpx pins Addr without a resolver.
		serviceName = ""
	}
	cfg := httpx.Config{
		ServiceName:    serviceName,
		Addr:           c.Addr,
		Discovery:      c.Discovery,
		Balancer:       c.Balancer,
		EjectThreshold: c.EjectThreshold,
		EjectFor:       c.EjectFor,
		Base:           base,
	}
	// Attach the executor and the fault + observe-resilience wrap hooks. Both
	// resilience and fault are resolved through neutral seams
	// ([resilience.ExecutorFor] / [fault.InjectorFor]) backed by the centralized
	// governance authority (starter-govern) when armed — nil-safe and a no-op
	// when governance/fault is off, so installing unconditionally is safe. Stack
	// order: observe( fault( rawExec ) ). exec is always non-nil (ExecutorFor
	// yields a no-op when governance is off), so fault always has an executor to
	// attach to; fault.WrapExecutor is nil-safe when no injector is registered.
	cfg.Executor = exec
	cfg.WrapExec = func(e resilience.Executor) resilience.Executor {
		return resilobserve.WrapExecutor(fault.WrapExecutor(e), "http", c.Observability)
	}
	return cfg
}
