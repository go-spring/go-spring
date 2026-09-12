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
	"context"
	"net/http"

	"go-spring.org/cloud/discovery"
	"go-spring.org/starter-http-client/httpx"
)

// Driver defines how to create one client entry's transport — THE extension
// point for customizing HTTP client assembly (the same shape starter-redigo
// uses). It is an OPTIONAL CONTAINER BEAN: a company may provide its own Driver
// bean (its constructor returns StarterHTTPClient.Driver); when none is
// present, starter-http-client falls back to the bundled [DefaultDriver] inside
// transport assembly. Because a custom driver is a bean, it may inject the
// configuration it needs at wiring time. Every configured entry is assembled
// through this one Driver; per-entry differences still reach it via the `name`
// and `c Config` it is called with.
//
// The driver owns the FULL assembly and returns the assembled RoundTripper
// together with the close function that releases its discovery watch and
// resilience executor. The bundled DefaultDriver simply delegates to
// [httpx.NewTransport] — the one-shot standard assembly (trace + TLS +
// discovery/LB + governance resilience + fault/observe + traffic). Two
// customization shapes:
//
//   - ADD to the default: embed DefaultDriver (or call httpx.NewTransport via
//     the standard config mapping), then wrap the returned RoundTripper with
//     your extras — auth headers, custom metrics, request filters.
//   - REPLACE: build the transport entirely your own way — e.g. call
//     [httpx.NewTransport] with a custom Base (proxy, pool tuning) or wrap the
//     whole chain; you simply own what the standard assembly would have done,
//     including the returned teardown.
//
// backend is the discovery backend the entry's ${discovery} label resolved to,
// already looked up by the starter wiring; it is nil when the entry cites no
// label (an unknown label fails at wiring, before the driver is called). It is
// passed as an argument rather than carried on Config so a custom driver can
// actually reach it — Config stays a pure bound value.
type Driver interface {
	CreateTransport(ctx context.Context, name string, c Config, backend discovery.Discovery) (rt http.RoundTripper, close func() error, err error)
}

// DefaultDriver is the default implementation of the Driver interface: the
// standard starter-http-client/httpx assembly driven by the bound Config.
type DefaultDriver struct{}

// CreateTransport maps the bound Config onto the starter-http-client/httpx assembler input
// and assembles the transport. Everything — trace, TLS surface,
// governance-resolved resilience executor, fault + observe wrap — is owned by
// starter-http-client/httpx; see [httpx.NewTransport].
func (DefaultDriver) CreateTransport(ctx context.Context, name string, c Config, backend discovery.Discovery) (http.RoundTripper, func() error, error) {
	return httpx.NewTransport(c.toTransportConfig(backend))
}
