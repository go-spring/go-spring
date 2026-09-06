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
// interface + the bundled DefaultDriver, which owns full client assembly
// (including service-discovery resolution).
package StarterElasticsearch

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create an Elasticsearch client. It is an
// OPTIONAL CONTAINER BEAN: a company or umbrella starter may provide its own
// Driver bean (its constructor returns StarterElasticsearch.Driver); when none
// is present, starter-elasticsearch falls back to the bundled [DefaultDriver]
// inside client assembly. A custom driver is a bean, so it may inject the
// configuration/beans it needs — e.g. company config bound from a properties
// file at wiring time.
//
// At most one Driver bean is expected per process; every client under
// ${spring.elasticsearch} is built through it, and per-instance differences
// are expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*elasticsearch.Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Elasticsearch client, bridged into go-spring's
// unified observability. Passing a nil provider to NewOtelInstrumentation makes
// the transport emit client spans through the OTel global TracerProvider that
// starter-otel installs; when starter-otel is absent that global is a no-op, so
// this stays a zero-config opt-in that needs no per-component adaptation.
//
// The transport is fixed at construction time and cannot be swapped on the
// client afterwards, and the resilience/observability policy is only injected
// into the wrapper after CreateClient returns. So CreateClient installs a thin
// [dynamicTransport] (an atomic RoundTripper indirection) whose behavior
// Init later swaps in — the observe+resilience transport built from
// the injected policy. The dynamic transport is tracked in [dynamicTransports]
// (keyed by the returned client) so newClient can hand it to the wrapper.
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*elasticsearch.Client, error) {
	dyn := newDynamicTransport()
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:              c.Addresses,
		Username:               c.Username,
		Password:               c.Password,
		APIKey:                 c.APIKey,
		ServiceToken:           c.ServiceToken,
		CloudID:                c.CloudID,
		CertificateFingerprint: c.CertificateFingerprint,
		MaxRetries:             c.MaxRetries,
		DisableRetry:           c.DisableRetry,
		CompressRequestBody:    c.CompressRequestBody,
		EnableMetrics:          c.EnableMetrics,
		EnableDebugLogger:      c.EnableDebugLogger,
		Instrumentation:        newOtelInstrumentation(),
		Transport:              dyn,
	})
	if err != nil {
		return nil, err
	}
	dynamicTransports.Store(client, dyn)
	return client, nil
}

// resolveAddresses resolves c.ServiceName through the registered discovery
// backend and returns the current live endpoint snapshot as "scheme://host:port"
// node addresses. Because the elasticsearch client exposes no dialer injection
// point this is a one-shot read at startup (the resolver has no background watch
// and no resources to release). It fails fast when no backend is registered or
// the service has no endpoints. It must only be called when service discovery
// is in effect (the caller has already gated on service-name being set and mesh
// mode being off).
func resolveAddresses(ctx context.Context, c Config) ([]string, error) {
	resolver, err := discovery.NewResolver(ctx, c.Discovery, c.ServiceName, discovery.WithScheme(c.Scheme))
	if err != nil {
		return nil, errutil.Explain(err, "elasticsearch: resolve service %s", c.ServiceName)
	}
	eps, err := resolver()
	if err != nil {
		return nil, errutil.Explain(err, "elasticsearch: resolve service %s", c.ServiceName)
	}
	if len(eps) == 0 {
		return nil, errutil.Explain(nil, "elasticsearch: discovery %q returned no endpoints for %q", c.Discovery, c.ServiceName)
	}
	addrs := make([]string, 0, len(eps))
	for _, ep := range eps {
		addrs = append(addrs, fmt.Sprintf("%s://%s", c.DiscoveryScheme, ep.Addr))
	}
	return addrs, nil
}
