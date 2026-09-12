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

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/tlsconf"
	"go-spring.org/starter-http-client/httpx"
)

// Config binds one declarative-HTTP-client entry under
// "${spring.http-client.instances.<name>}". Each entry contributes a route whose transport is
// assembled by stdlib/httpx: service discovery + load balancing when
// ServiceName is set, or a fixed address otherwise, optionally protected by
// resilience and always traced through the OTel globals. All entries are served
// by one process-wide httpclt.DoRequest replacement; a generated client routes
// by its Target (addr or service-name), so switching a call between a direct
// address and a discovered service is a pure-config change.
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

	// Discovery names the discovery backend bean that resolves ServiceName.
	// Required when ServiceName is set; the starter wiring resolves this label
	// against every registered discovery backend bean and hands the result to
	// the driver as the Discovery argument of CreateTransport.
	Discovery string `value:"${discovery:=}"`

	// TLS configures the certificate surface for https targets: a client key
	// pair (tls.cert-file/key-file), a CA bundle (tls.ca-file), the expected
	// peer name (tls.server-name) and an insecure escape hatch. Off by default;
	// when enabled it is wired into the transport's TLS config, so
	// WithScheme("https") gets verifiable TLS instead of system defaults.
	TLS tlsconf.TLSConfig `value:"${tls:=}"`
}

// Resilience, fault and endpoint-selection policy are NOT bound here: they live
// in the governance rules document (starter-governance), matched by this
// entry's resource label. That includes the load-balancing strategy
// (`balancer`) and outlier suspension (`outlier-threshold` /
// `outlier-suspend-for`) — write them under govern.rules[N], not here.
//
// Keep govern retry counts at 0 unless requests are idempotent: the client may
// issue POSTs and other non-idempotent verbs, and a retry re-sends them.

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

// toTransportConfig maps the bound Config onto the starter-http-client/httpx assembler input.
// backend is the discovery backend the starter wiring resolved from the
// Discovery label (nil when the entry does not cite one). Everything — TLS
// surface, trace layer, governance-resolved resilience executor, fault +
// observe wrap, base dialer — is implemented by starter-http-client/httpx.
func (c Config) toTransportConfig(backend discovery.Discovery) httpx.Config {
	serviceName := c.ServiceName
	if c.Addr != "" {
		// Direct addressing: service-name (when set) is a pure governance label,
		// NOT a discovery target — blank it so httpx pins Addr without a resolver.
		// Resource keeps the original service-name so the governance label stays
		// stable across addressing-mode switches.
		serviceName = ""
	}
	resource := resilience.ResourceLabel("http", c.ServiceName, c.Addr)
	return httpx.Config{
		ServiceName: serviceName,
		Addr:        c.Addr,
		Discovery:   backend,
		TLS:         c.TLS,
		Resource:    resource,
	}
}
