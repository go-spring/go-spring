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
// injected HTTP client carrying the dynamic transport that Init later arms).
// It mirrors starter-elasticsearch's driver.go.
package StarterInfluxdb

import (
	"context"
	"net/http"
	"sync"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

// Driver interface defines how to create an InfluxDB client (an
// influxdb2.Client). It is an OPTIONAL CONTAINER BEAN: a company or umbrella
// starter may provide its own Driver bean (its constructor returns
// StarterInfluxdb.Driver); when none is present, starter-influxdb falls back to
// the bundled [DefaultDriver] inside client assembly. A custom driver is a
// bean, so it may inject the configuration/beans it needs — e.g. company config
// bound from a properties file at wiring time.
//
// At most one Driver bean is expected per process; every client under
// ${spring.influxdb} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config) (influxdb2.Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new influxdb2.Client from the provided
// configuration.
//
// The HTTP client is injected through Options so requests ride the
// dynamicTransport — an atomic RoundTripper indirection whose behavior Init
// later swaps in (the observe+resilience transport built from the injected
// policy). The dynamic transport is tracked in [dynamicTransports] (keyed by
// the returned client) so newClient can hand it to the wrapper.
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (influxdb2.Client, error) {
	dyn := newDynamicTransport()
	opts := influxdb2.DefaultOptions().SetHTTPClient(&http.Client{Transport: dyn})
	cl := influxdb2.NewClientWithOptions(c.ServerURL, c.AuthToken, opts)
	dynamicTransports.Store(cl, dyn)
	return cl, nil
}

// dynamicTransports tracks the dynamic transport DefaultDriver installed for
// each client, so newClient can hand it to the wrapper for Init to arm. The
// key is the influxdb2.Client value; only clients built by DefaultDriver
// appear here.
var dynamicTransports sync.Map // influxdb2.Client -> *dynamicTransport
