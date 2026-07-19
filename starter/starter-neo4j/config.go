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

package StarterNeo4j

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/auth"
	"go-spring.org/stdlib/starter"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Config defines Neo4j connection configuration.
type Config struct {
	// URI is the Neo4j connection URI, e.g., "neo4j://127.0.0.1:7687" or
	// "bolt://127.0.0.1:7687". The scheme selects routing and encryption.
	URI string `value:"${uri}"`

	// Username is the Neo4j username. When empty, the client connects with no
	// authentication.
	Username string `value:"${username:=}"`

	// Password is the Neo4j password, default is empty.
	Password string `value:"${password:=}"`

	// Realm is the authentication realm, default is empty.
	Realm string `value:"${realm:=}"`

	// MaxConnectionPoolSize is the maximum number of connections per host held
	// by the connection pool.
	MaxConnectionPoolSize int `value:"${max-connection-pool-size:=100}"`

	// MaxConnectionLifetime is the maximum amount of time a connection can be
	// reused before it is retired, e.g., "1h".
	MaxConnectionLifetime time.Duration `value:"${max-connection-lifetime:=1h}"`

	// ConnectionAcquisitionTimeout is the maximum time to wait for a connection
	// from the pool, e.g., "1m".
	ConnectionAcquisitionTimeout time.Duration `value:"${connection-acquisition-timeout:=1m}"`

	// SocketConnectTimeout is the timeout for establishing the TCP connection,
	// e.g., "5s".
	SocketConnectTimeout time.Duration `value:"${socket-connect-timeout:=5s}"`

	// MaxTransactionRetryTime is the maximum time transactional functions retry
	// on transient errors, e.g., "30s".
	MaxTransactionRetryTime time.Duration `value:"${max-transaction-retry-time:=30s}"`

	// TLS configures the certificate trust used for the encrypted URI schemes
	// ("bolt+s", "bolt+ssc", "neo4j+s", "neo4j+ssc"). Encryption itself is
	// selected by the URI scheme; these fields only customize the trust store
	// and client certificate. They are ignored for the plaintext "bolt"/"neo4j"
	// schemes.
	TLS starter.TLSConfig `value:"${tls}"`

	// Driver specifies which Neo4j driver to use, defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`

	// ServiceName resolves the connection address through a registered discovery
	// backend instead of relying solely on the URI host. When set, the endpoint
	// is resolved once at startup and spliced into the URI host, so the driver
	// connects to a live instance handed out by the company naming service.
	//
	// Limitation: unlike clients that accept a custom dialer, the neo4j driver
	// builds its connection pool from the URI and exposes no dialer injection
	// point, so this is a one-shot resolution at startup — address changes after
	// startup are not picked up until the client is rebuilt. When empty, the URI
	// host is used unchanged.
	ServiceName string `value:"${service-name:=}"`

	// Discovery selects which registered discovery backend resolves ServiceName.
	// It is only consulted when ServiceName is set. A company registers its
	// naming service once via discovery.Register; the default backend name is
	// "default".
	Discovery string `value:"${discovery:=default}"`
}

// applyTLS configures the encryption-related fields of conf from the shared TLS
// settings. The CA certificate (if any) is loaded into conf.TlsConfig, and a
// client certificate (if any) is installed as a static certificate provider for
// mutual TLS. Both only take effect for the "+s"/"+ssc" URI schemes.
func applyTLS(t starter.TLSConfig, conf *neo4j.Config) error {
	if t.CAFile != "" {
		pem, err := os.ReadFile(t.CAFile)
		if err != nil {
			return fmt.Errorf("neo4j: read ca cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return fmt.Errorf("neo4j: no certificates parsed from %s", t.CAFile)
		}
		conf.TlsConfig = &tls.Config{RootCAs: pool}
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

// Driver interface defines how to create a Neo4j client.
type Driver interface {
	CreateClient(c Config) (neo4j.DriverWithContext, error)
}

// RegisterDriver registers a Neo4j driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("neo4j driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Neo4j client based on the provided configuration.
func (DefaultDriver) CreateClient(c Config) (neo4j.DriverWithContext, error) {
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
