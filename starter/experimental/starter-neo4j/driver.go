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
// (including TLS and service-discovery resolution).

package StarterNeo4j

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/url"
	"os"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/auth"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/security"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a Neo4j client. It is an OPTIONAL
// CONTAINER BEAN: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterNeo4j.Driver); when none is present,
// starter-neo4j falls back to the bundled [DefaultDriver] inside client
// assembly. A custom driver is a bean, so it may inject the configuration/beans
// it needs — e.g. company config bound from a properties file at wiring time.
//
// CreateClient returns the module's exported [Client] — the wrapper apps inject
// — not the raw neo4j.DriverWithContext, so a driver takes part in the type the
// rest of the ecosystem sees and future wrapper capabilities are reachable from
// it. It returns the client COMPLETE: the wrapper derives the service label from
// the Config it is handed, so the driver fixes identity here; center is the
// container's runtime (carrying the resilience/fault/loadbalance
// authorities and the discovery directory), which [NewClient] applies while
// building. Nothing patches the client afterwards.
//
// center is one object rather than a parameter per capability so this interface
// — which every company driver implements — stays stable as capabilities are
// added. A driver that has no use for one of its facilities simply ignores it.
//
// At most one Driver bean is expected per process; every client under
// ${spring.neo4j} is built through it, and per-instance differences are
// expressed through [Config].
//
// The entry's ${discovery} label rides on the config (c.Discovery, already
// carrying the family default): the driver resolves the backend it names from
// center.Directory itself, failing construction when an entry routes by
// service-name and the label names no bean.
type Driver interface {
	CreateClient(ctx context.Context, c Config, center *governance.Runtime) (*Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Neo4j client based on the provided configuration.
//
// When in-process discovery applies (a service name is set and the process is
// not in mesh mode), the discovery backend the entry's ${discovery} label names
// is resolved from center.Directory and ONE eligible endpoint is picked as the
// routing driver's SEED address and spliced into the URI host. This is seed
// resolution, not load balancing: the pool is built over that address for the
// driver's lifetime, and every later address decision belongs to the driver's
// routing table. The same resolver feeds the driver's AddressResolver hook —
// the fallback the routing driver consults after both its last-known routers
// and the seeded router failed — so a client can re-find the cluster. An entry
// routed by service-name that cites no existing backend fails here, at
// construction. [Config.Validate] has already rejected a service name paired
// with a direct (bolt) scheme.
func (DefaultDriver) CreateClient(ctx context.Context, c Config, center *governance.Runtime) (*Client, error) {
	// A hand-built config bypasses the binder, so the same scheme × service-name
	// rule is enforced here too; the normal path has already passed it.
	if err := c.Validate(); err != nil {
		return nil, err
	}

	// Seed the URI host from the discovery backend this entry cites. The label
	// rides on the config (c.Discovery, already carrying the family default);
	// a missing backend fails loudly.
	useDiscovery := center.UsesDiscovery(c.ServiceName)
	var resolve discovery.Resolver
	if useDiscovery {
		backend, _ := center.Directory.Get(c.Discovery)
		if backend == nil {
			if c.Discovery == "" {
				return nil, errutil.Explain(nil, "neo4j: instance routes by service-name but sets no discovery backend (set spring.neo4j.instances.<name>.discovery to the name of a discovery backend bean)")
			}
			return nil, errutil.Explain(nil, "neo4j: discovery backend %q not found (no discovery.Discovery bean with this name; cited by the entry's ${discovery} label)", c.Discovery)
		}
		var err error
		resolve, err = discovery.NewResolver(ctx, backend, c.ServiceName, c.Scheme)
		if err != nil {
			return nil, errutil.Explain(err, "neo4j: resolve service %s", c.ServiceName)
		}
		seed, err := resolveSeed(ctx, resolve, c.ServiceName)
		if err != nil {
			return nil, err
		}
		c.URI, err = withHost(c.URI, seed)
		if err != nil {
			return nil, err
		}
	}

	auth := neo4j.NoAuth()
	if c.Username != "" {
		auth = neo4j.BasicAuth(c.Username, c.Password, c.Realm)
	}
	var tlsErr error
	client, err := neo4j.NewDriverWithContext(c.URI, auth, func(conf *neo4j.Config) {
		conf.MaxConnectionPoolSize = c.MaxConnectionPoolSize
		conf.MaxConnectionLifetime = c.MaxConnectionLifetime
		conf.ConnectionAcquisitionTimeout = c.ConnectionAcquisitionTimeout
		conf.SocketConnectTimeout = c.SocketConnectTimeout
		conf.MaxTransactionRetryTime = c.MaxTransactionRetryTime
		tlsErr = applyTLS(c.TLS, conf)
		// Discovery in effect: hand the routing driver a live router set to retry
		// with, so a client whose seeded host disappears can find the cluster
		// again instead of staying broken until the process restarts. The driver
		// consults this hook only after both its last-known routers and the
		// seeded root router failed.
		if resolve != nil {
			conf.AddressResolver = liveRouterAddresses(resolve)
		}
	})
	if err != nil {
		return nil, err
	}
	if tlsErr != nil {
		return nil, tlsErr
	}
	// NewClient is the only way to build a Client: identity and governance are
	// both applied here (see [Driver]), so the driver returns a client that is
	// complete.
	return NewClient(client, c, center), nil
}

// resolveSeed returns the one address the routing driver is dialed with. It is
// seed resolution, not load balancing: the driver builds its pool over this
// address and owns every later address decision through its routing table.
// Choosing among instances once, at construction, would be balancing in name
// only — the choice is never revisited.
//
// The eligibility order follows the endpoint model: a disabled instance never
// receives traffic; healthy instances are preferred, degrading to all
// non-disabled when none is healthy; a zero weight is the naming service's
// drain signal and is skipped unless every eligible candidate is draining.
func resolveSeed(ctx context.Context, resolve discovery.Resolver, service string) (string, error) {
	eps, err := resolve(ctx)
	if err != nil {
		return "", errutil.Explain(err, "neo4j: resolve service %s", service)
	}
	if ep, ok := seedEndpoint(eps); ok {
		return ep.Addr, nil
	}
	return "", errutil.Explain(nil, "neo4j: service %s resolved no eligible endpoint", service)
}

// seedEndpoint picks the seed from eps in the eligibility order resolveSeed
// documents. It returns false when every endpoint is disabled (or eps is
// empty) — there is no address that may receive traffic.
func seedEndpoint(eps []discovery.Endpoint) (discovery.Endpoint, bool) {
	first := func(accept func(discovery.Endpoint) bool) (discovery.Endpoint, bool) {
		for _, ep := range eps {
			if !ep.Disabled && accept(ep) {
				return ep, true
			}
		}
		return discovery.Endpoint{}, false
	}
	if ep, ok := first(func(ep discovery.Endpoint) bool { return ep.Healthy && ep.Weight != 0 }); ok {
		return ep, true
	}
	if ep, ok := first(func(ep discovery.Endpoint) bool { return ep.Healthy }); ok {
		return ep, true
	}
	if ep, ok := first(func(ep discovery.Endpoint) bool { return ep.Weight != 0 }); ok {
		return ep, true
	}
	return first(func(discovery.Endpoint) bool { return true })
}

// withHost rewrites the URI's host to addr, preserving scheme, credentials and
// query. It is how the resolved seed replaces the entry's placeholder host.
func withHost(uri, addr string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", errutil.Explain(err, "neo4j: parse uri %s", uri)
	}
	u.Host = addr
	return u.String(), nil
}

// liveRouterAddresses adapts a discovery [discovery.Resolver] into the driver's
// [neo4j.ServerAddressResolver] hook: the router set the routing driver retries
// with after both its last-known routers and the seeded root router failed to
// yield a routing table. That is exactly the recovery a discovery-backed client
// wants — the instance baked into the URI is gone, ask the naming service where
// the cluster is now — and it is the driver's ONLY resolver seam, so it is what
// a direct client cannot use.
//
// On a read error or an empty (or entirely disabled) snapshot it returns the
// address the driver already had, so a registry hiccup leaves the driver
// exactly as it was rather than handing it an empty router list to fail on.
func liveRouterAddresses(resolve discovery.Resolver) neo4j.ServerAddressResolver {
	// The driver's address-resolver hook carries no context, so this is one of
	// the adapter boundaries that supplies context.Background() itself.
	return func(current neo4j.ServerAddress) []neo4j.ServerAddress {
		eps, err := resolve(context.Background())
		if err != nil {
			return []neo4j.ServerAddress{current}
		}
		routers := make([]neo4j.ServerAddress, 0, len(eps))
		for _, ep := range eps {
			if ep.Disabled {
				continue
			}
			host, port, err := net.SplitHostPort(ep.Addr)
			if err != nil {
				continue
			}
			routers = append(routers, neo4j.NewServerAddress(host, port))
		}
		if len(routers) == 0 {
			return []neo4j.ServerAddress{current}
		}
		return routers
	}
}

// applyTLS configures the encryption-related fields of conf from the shared TLS
// settings. The CA certificate (if any) is loaded into conf.TlsConfig, and a
// client certificate (if any) is installed as a static certificate provider for
// mutual TLS. Both only take effect for the "+s"/"+ssc" URI schemes — Enabled
// does not control encryption here (the URI scheme does), it is a placeholder
// kept for config-shape parity.
func applyTLS(t security.TLSConfig, conf *neo4j.Config) error {
	if t.CAFile != "" || t.ServerName != "" || t.InsecureSkipVerify {
		conf.TlsConfig = &tls.Config{}
	}
	if t.ServerName != "" {
		conf.TlsConfig.ServerName = t.ServerName
	}
	if t.InsecureSkipVerify {
		conf.TlsConfig.InsecureSkipVerify = true
	}
	if t.CAFile != "" {
		pem, err := os.ReadFile(t.CAFile)
		if err != nil {
			return errutil.Explain(err, "neo4j: read ca cert")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return errutil.Explain(nil, "neo4j: no certificates parsed from %s", t.CAFile)
		}
		conf.TlsConfig.RootCAs = pool
	}
	if t.CertFile != "" || t.KeyFile != "" {
		provider, err := auth.NewStaticClientCertificateProvider(
			auth.ClientCertificate{CertFile: t.CertFile, KeyFile: t.KeyFile},
		)
		if err != nil {
			return errutil.Explain(err, "neo4j: load client certificate")
		}
		conf.ClientCertificateProvider = provider
	}
	return nil
}
