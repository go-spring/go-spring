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

// driver.go is the "construction seam" concept of this starter: the Driver
// interface + the bundled DefaultDriver, which owns full client assembly (the
// injected HTTP client carrying the dynamic transport that [NewClient] fills).
// It mirrors starter-elasticsearch's driver.go.
package StarterInfluxdb

import (
	"context"
	"net/http"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"go-spring.org/cloud"
)

// Driver interface defines how to create an InfluxDB client. It is an OPTIONAL
// CONTAINER BEAN: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterInfluxdb.Driver); when none is present,
// starter-influxdb falls back to the bundled [DefaultDriver] inside client
// assembly. A custom driver is a bean, so it may inject the configuration/beans
// it needs — e.g. company config bound from a properties file at wiring time.
//
// CreateClient returns the module's exported [Client] — the wrapper apps inject
// — not the raw influxdb2.Client, so a driver takes part in the type the rest
// of the ecosystem sees and future wrapper capabilities are reachable from it.
// It returns the client COMPLETE: a custom driver builds its raw client however
// it likes and hands it to [NewClient] together with params, which supplies the
// container's facilities (see [cloud.ClientParams]) and is applied while the
// client is built. Nothing patches the client afterwards.
//
// params is one struct rather than a parameter per capability so this interface
// — which every company driver implements — stays stable as capabilities are
// added. A driver that has no use for one of its fields simply ignores it.
//
// At most one Driver bean is expected per process; every client under
// ${spring.influxdb} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new client from the provided configuration.
//
// The HTTP client is injected through Options so requests ride the
// dynamicTransport — a RoundTripper indirection [NewClient] swaps the
// declaration+resilience transport into while the client is built (from
// params' executor). The indirection is handed to [NewClient] so the wrapper
// can reach it.
func (DefaultDriver) CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error) {
	dyn := newDynamicTransport()
	opts := influxdb2.DefaultOptions().SetHTTPClient(&http.Client{Transport: dyn})
	cl := influxdb2.NewClientWithOptions(c.ServerURL, c.AuthToken, opts)
	return NewClient(cl, dyn, c, params), nil
}
