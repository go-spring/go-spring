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

package StarterMongoDB

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"
)

// Config defines MongoDB client connection configuration.
type Config struct {
	// URI is the MongoDB connection string,
	// e.g., "mongodb://127.0.0.1:27017".
	URI string `value:"${uri}"`

	// Username is the username for authentication. When empty, credentials are
	// taken solely from the URI (if any). Default is empty.
	Username string `value:"${username:=}"`

	// Password is the password for authentication, default is empty.
	Password string `value:"${password:=}"`

	// AuthSource is the database against which credentials are verified,
	// e.g., "admin". Default is empty (driver default).
	AuthSource string `value:"${auth-source:=}"`

	// AuthMechanism is the authentication mechanism, e.g., "SCRAM-SHA-256".
	// Default is empty (driver negotiates automatically).
	AuthMechanism string `value:"${auth-mechanism:=}"`

	// ConnectTimeout is the timeout for establishing the initial connection,
	// 0 uses the driver default, e.g., "10s".
	ConnectTimeout time.Duration `value:"${connect-timeout:=10s}"`

	// ServerSelectionTimeout bounds how long the driver waits to find a suitable
	// server before failing, 0 uses the driver default, e.g., "30s".
	ServerSelectionTimeout time.Duration `value:"${server-selection-timeout:=0}"`

	// MaxPoolSize is the maximum number of connections in the pool,
	// 0 uses the driver default, e.g., "100".
	MaxPoolSize uint64 `value:"${max-pool-size:=100}"`

	// MinPoolSize is the minimum number of connections in the pool, default is 0.
	MinPoolSize uint64 `value:"${min-pool-size:=0}"`

	// MaxConnIdleTime is the maximum time a connection may remain idle in the
	// pool before being closed, 0 means no limit, e.g., "5m".
	MaxConnIdleTime time.Duration `value:"${max-conn-idle-time:=0}"`

	// TLS configures transport encryption for the connection.
	TLS TLSConfig `value:"${tls}"`
}

// TLSConfig configures TLS for the MongoDB client. When Enabled is false all
// other fields are ignored and the connection is made in plaintext (unless the
// URI itself requests TLS).
type TLSConfig struct {
	// Enabled turns on TLS for the connection, default is false.
	Enabled bool `value:"${enabled:=false}"`

	// CACertFile is the path to a PEM-encoded CA certificate used to verify the
	// server certificate. When empty, the system trust store is used.
	CACertFile string `value:"${ca-cert-file:=}"`

	// CertFile is the path to the PEM-encoded client certificate for mutual TLS,
	// default is empty.
	CertFile string `value:"${cert-file:=}"`

	// KeyFile is the path to the PEM-encoded client private key for mutual TLS,
	// default is empty.
	KeyFile string `value:"${key-file:=}"`

	// InsecureSkipVerify disables server certificate verification. It should be
	// used only for testing, default is false.
	InsecureSkipVerify bool `value:"${insecure-skip-verify:=false}"`
}

// build constructs a *tls.Config from the TLS settings, loading the CA and
// client certificates from disk when configured. It returns nil when TLS is
// disabled.
func (t TLSConfig) build() (*tls.Config, error) {
	if !t.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{InsecureSkipVerify: t.InsecureSkipVerify}
	if t.CACertFile != "" {
		pem, err := os.ReadFile(t.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("mongodb: read ca cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("mongodb: no certificates parsed from %s", t.CACertFile)
		}
		cfg.RootCAs = pool
	}
	if t.CertFile != "" || t.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("mongodb: load client key pair: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}
