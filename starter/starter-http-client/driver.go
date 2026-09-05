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

	"go-spring.org/cloud/httpx"
)

// driverRegistry maps driver names to their implementations. The bundled
// DefaultDriver is registered at init; custom drivers add themselves via
// RegisterDriver (e.g. from an init in the application).
var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("default", DefaultDriver{})
}

// Driver defines how to create one client entry's transport — THE extension
// point for customizing HTTP client assembly (the same shape starter-redigo
// uses). A company (or the bundled DefaultDriver) implements it once and
// registers via RegisterDriver; callers select one through Config.Driver, which
// defaults to "default".
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
type Driver interface {
	CreateTransport(ctx context.Context, name string, c Config) (rt http.RoundTripper, close func() error, err error)
}

// RegisterDriver registers an HTTP client driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("http client driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface: the
// standard cloud/httpx assembly driven by the bound Config.
type DefaultDriver struct{}

// CreateTransport maps the bound Config onto the cloud/httpx assembler input
// and assembles the transport. Everything — trace, TLS surface,
// governance-resolved resilience executor, fault + observe wrap — is owned by
// cloud/httpx; see [httpx.NewTransport].
func (DefaultDriver) CreateTransport(ctx context.Context, name string, c Config) (http.RoundTripper, func() error, error) {
	return httpx.NewTransport(c.toTransportConfig())
}
