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

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/elastic/go-elasticsearch/v8"
	"go-spring.org/cloud"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/mesh"
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
// CreateClient returns the module's exported [Client] — the wrapper apps inject
// — not the raw *elasticsearch.Client, so a driver takes part in the type the
// rest of the ecosystem sees. It returns the client COMPLETE: cfg fixes the
// addresses/cloud-id/service-name the resilience service label is derived from,
// the driver hands the wrapper the [dynamicTransport] it installed so the
// declaration transport can be swapped in, and params supplies the container's
// facilities (see [cloud.ClientParams]), which [NewClient] applies while
// building. Nothing patches the client afterwards.
//
// params is one struct rather than a parameter per capability so this interface
// — which every company driver implements — stays stable as capabilities are
// added. A driver that has no use for one of its fields simply ignores it.
//
// At most one Driver bean is expected per process; every client under
// ${spring.elasticsearch} is built through it, and per-instance differences
// are expressed through [Config].
//
// params.Discovery is the discovery backend the entry's ${discovery} label
// resolved to, already looked up by the starter wiring; it is nil when no
// backend bean exists. It rides on the params struct rather than Config so a
// custom driver can actually reach it — Config stays a pure bound value.
type Driver interface {
	CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Elasticsearch client whose requests are declared to
// go-spring's unified observability. The transport carries no instrumentation of
// its own — the elastic transport's OTel instrumentation is deliberately NOT
// enabled, because it emitted a call-level client span per request that
// duplicated the one the resilience layer now opens; the declaration + emission
// pair lives entirely on the wrapper's transport stack ([NewClient]).
//
// The transport is fixed at construction time and cannot be swapped on the
// client afterwards, while the declaration+resilience transport is built by the
// wrapper's constructor ([NewClient]). So CreateClient installs a thin
// [dynamicTransport] (a mutex-guarded RoundTripper indirection) as the client's
// transport and returns a wrapper built over it: [NewClient] swaps the
// declaration+resilience stack in, with the indirection keeping it installable
// after elasticsearch.NewClient has already captured the transport.
func (DefaultDriver) CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error) {
	dyn := newDynamicTransport()
	cfg := elasticsearch.Config{
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
		Transport:              dyn,
	}
	// Service discovery in effect: replace the transport's static node set with a
	// live one, so cluster membership follows the naming service instead of the
	// boot-time snapshot. Transport node selection is untouched — the live pool
	// keeps a library-built inner pool and only rebuilds it when the node set
	// changes. Without discovery the static Addresses stand as configured.
	if c.ServiceName != "" && params.Discovery != nil && !mesh.Enabled() {
		resolver, err := discovery.NewResolver(ctx, params.Discovery, c.ServiceName, discovery.WithScheme(c.Scheme))
		if err != nil {
			return nil, errutil.Explain(err, "elasticsearch: resolve service %s", c.ServiceName)
		}
		if resolver != nil {
			cfg.ConnectionPoolFunc = func(conns []*elastictransport.Connection, sel elastictransport.Selector) elastictransport.ConnectionPool {
				return newLivePool(resolver, c.DiscoveryScheme, sel, conns)
			}
		}
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return NewClient(client, dyn, c, params), nil
}

// resolveAddresses resolves c.ServiceName through the discovery backend the
// starter wiring resolved from c.Discovery and returns the current live endpoint
// snapshot as "scheme://host:port" node addresses. It is a single read: the
// fail-fast gate and the seed for the live connection pool (see the
// ConnectionPoolFunc in [DefaultDriver.CreateClient]), which is what keeps the
// node set following the naming service afterwards. It fails
// fast when backend is nil (no backend bean cited by the ${discovery} label) or
// the service has no endpoints. It must only be called when service discovery is
// in effect (the caller has already gated on service-name being set and mesh
// mode being off).
func resolveAddresses(ctx context.Context, c Config, backend discovery.Discovery) ([]string, error) {
	if backend == nil {
		if c.Discovery == "" {
			return nil, errutil.Explain(nil, "elasticsearch: instance routes by service-name but sets no discovery backend (set spring.elasticsearch.instances.<name>.discovery to the name of a discovery backend bean)")
		}
		return nil, errutil.Explain(nil, "elasticsearch: discovery backend %q not found (no discovery.Discovery bean with this name; cited by the entry's ${discovery} label)", c.Discovery)
	}
	resolver, err := discovery.NewResolver(ctx, backend, c.ServiceName, discovery.WithScheme(c.Scheme))
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
