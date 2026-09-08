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
	"fmt"
	"net/url"
	"os"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/auth"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/tlsconf"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a Neo4j client. It is an OPTIONAL
// CONTAINER BEAN: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterNeo4j.Driver); when none is present,
// starter-neo4j falls back to the bundled [DefaultDriver] inside client
// assembly. A custom driver is a bean, so it may inject the configuration/beans
// it needs — e.g. company config bound from a properties file at wiring time.
//
// At most one Driver bean is expected per process; every client under
// ${spring.neo4j} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config) (neo4j.DriverWithContext, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Neo4j client based on the provided configuration.
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (neo4j.DriverWithContext, error) {
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
	})
	if err != nil {
		return nil, err
	}
	if tlsErr != nil {
		return nil, tlsErr
	}
	return client, nil
}

// applyTLS configures the encryption-related fields of conf from the shared TLS
// settings. The CA certificate (if any) is loaded into conf.TlsConfig, and a
// client certificate (if any) is installed as a static certificate provider for
// mutual TLS. Both only take effect for the "+s"/"+ssc" URI schemes — Enabled
// does not control encryption here (the URI scheme does), it is a placeholder
// kept for config-shape parity.
func applyTLS(t tlsconf.TLSConfig, conf *neo4j.Config) error {
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
			return fmt.Errorf("neo4j: read ca cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return fmt.Errorf("neo4j: no certificates parsed from %s", t.CAFile)
		}
		conf.TlsConfig.RootCAs = pool
	}
	if t.CertFile != "" || t.KeyFile != "" {
		provider, err := auth.NewStaticClientCertificateProvider(
			auth.ClientCertificate{CertFile: t.CertFile, KeyFile: t.KeyFile},
		)
		if err != nil {
			return fmt.Errorf("neo4j: load client certificate: %w", err)
		}
		conf.ClientCertificateProvider = provider
	}
	return nil
}

// resolveURI resolves c.ServiceName through the injected discovery backend,
// picks one live endpoint via the shared loadbalance machinery, and rewrites
// the URI's host to that address. It must only be called when service discovery
// is in effect (the caller has already gated on service-name being set and mesh
// mode being off); it fails loudly when no backend bean was injected for the
// ${discovery} label.
func resolveURI(ctx context.Context, c Config) (string, error) {
	if c.backend == nil {
		return "", errutil.Explain(nil, "neo4j: discovery backend %q not found (no discovery.Discovery bean with this name; cited by the entry's ${discovery} label)", c.Discovery)
	}
	resolver, err := discovery.NewResolver(ctx, c.backend, c.ServiceName, discovery.WithScheme(c.Scheme))
	if err != nil {
		return "", errutil.Explain(err, "neo4j: resolve service %s", c.ServiceName)
	}
	eps, err := resolver()
	if err != nil {
		return "", errutil.Explain(err, "neo4j: pick endpoint for %s", c.ServiceName)
	}
	bal, err := loadbalance.New(loadbalance.RoundRobin)
	if err != nil {
		return "", errutil.Explain(err, "neo4j: balancer for %s", c.ServiceName)
	}
	ep, err := loadbalance.NewPool(loadbalance.SourceFunc(func() ([]discovery.Endpoint, error) {
		return eps, nil
	}), bal).Pick(loadbalance.PickInfo{})
	if err != nil {
		return "", errutil.Explain(err, "neo4j: pick endpoint for %s", c.ServiceName)
	}
	u, err := url.Parse(c.URI)
	if err != nil {
		return "", errutil.Explain(err, "neo4j: parse uri %s", c.URI)
	}
	u.Host = ep.Addr
	return u.String(), nil
}
