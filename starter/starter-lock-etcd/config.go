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

package StarterLockEtcd

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	"go-spring.org/stdlib/errutil"
)

// Config binds one etcd-backed distributed-lock instance under
// spring.lock.<name>. Endpoints is required; every other field has a sensible
// default so a minimal configuration only needs the cluster address.
type Config struct {
	// Endpoints lists the etcd cluster nodes to dial. Required; an empty list
	// is rejected at startup so a misconfigured instance never boots silently.
	Endpoints []string `value:"${endpoints}"`

	// Username / Password authenticate against etcd when auth is enabled.
	// Leave empty for anonymous clusters.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// DialTimeout bounds the initial connection attempt. It also bounds the
	// startup readiness probe used to fail fast on unreachable clusters.
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// TTL is the lease duration attached to each acquired lock. When the
	// holder crashes the lease expires after roughly TTL and the lock becomes
	// available. etcd sessions use whole-second TTLs, so values below one
	// second are rounded up to one second.
	TTL time.Duration `value:"${ttl:=30s}"`

	// KeyPrefix is prepended to every lock key so multiple applications can
	// share one cluster without collisions. Trailing slashes are preserved.
	KeyPrefix string `value:"${key-prefix:=/lock/}"`

	// TLS configures optional transport-layer security. Off by default.
	TLS TLSConfig `value:"${tls}"`
}

// TLSConfig configures a TLS connection to etcd. It is only applied when
// Enabled is true.
type TLSConfig struct {
	// Enabled turns on TLS for the connection.
	Enabled bool `value:"${enabled:=false}"`

	// CertFile and KeyFile are the client certificate/key pair used for
	// mutual TLS. Leave both empty when the server does not require a
	// client cert.
	CertFile string `value:"${cert-file:=}"`
	KeyFile  string `value:"${key-file:=}"`

	// CACertFile is a PEM bundle of root CAs used to verify the server
	// certificate. When empty the host's default root set is used.
	CACertFile string `value:"${ca-cert-file:=}"`
}

// ttlSeconds returns the session TTL in whole seconds, clamped to a minimum of
// one second. The etcd concurrency package refuses TTLs below one second.
func (c Config) ttlSeconds() int {
	d := c.TTL
	if d <= 0 {
		d = 30 * time.Second
	}
	s := int(d / time.Second)
	if d%time.Second != 0 {
		s++
	}
	if s < 1 {
		s = 1
	}
	return s
}

// buildTLSConfig turns a TLSConfig into a *tls.Config, or nil when TLS is
// disabled. It loads the client key pair and CA bundle from disk when
// provided, mirroring the pattern used by other Go-Spring starters.
func buildTLSConfig(c TLSConfig) (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{}
	if c.CertFile != "" || c.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, errutil.Explain(err, "lock-etcd: failed to load TLS key pair")
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if c.CACertFile != "" {
		pem, err := os.ReadFile(c.CACertFile)
		if err != nil {
			return nil, errutil.Explain(err, "lock-etcd: failed to read TLS CA file")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errutil.Explain(nil, "lock-etcd: no certificates found in CA file %s", c.CACertFile)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}
