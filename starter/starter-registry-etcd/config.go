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

package StarterRegistryEtcd

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	"go-spring.org/stdlib/errutil"
)

// EtcdConfig binds the etcd cluster connection under ${spring.registry.etcd}.
type EtcdConfig struct {
	// Endpoints lists the etcd cluster nodes to dial. Required; setting it is
	// what activates this starter (fail-loud opt-in; no silent localhost).
	Endpoints []string `value:"${endpoints}"`

	// Username / Password authenticate against etcd when auth is enabled.
	// Leave empty for anonymous clusters.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// DialTimeout bounds the initial connection attempt. It also bounds the
	// startup readiness probe used to fail fast on unreachable clusters.
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// TTL is the lease duration attached to a registered instance. The registrar
	// keeps the lease alive on a background keep-alive while the instance is up;
	// if the process dies the lease expires after roughly TTL and etcd removes
	// the key. etcd leases use whole-second TTLs, so sub-second values are
	// rounded up to one second.
	TTL time.Duration `value:"${ttl:=15s}"`

	// KeyPrefix is prepended to every registered key so multiple applications can
	// share one cluster without collisions. Trailing slashes are preserved.
	KeyPrefix string `value:"${key-prefix:=/services/}"`

	// TLS configures optional transport-layer security. Off by default.
	TLS TLSConfig `value:"${tls}"`

	// Name is the key this etcd registrar is published under in the
	// stdlib/discovery registrar registry. The register server selects a backend
	// by this name via ${spring.registry.backend}; keep both at "default" for the
	// common single-registry case.
	Name string `value:"${name:=default}"`
}

// TLSConfig configures a TLS connection to etcd. It is only applied when
// Enabled is true.
type TLSConfig struct {
	// Enabled turns on TLS for the connection.
	Enabled bool `value:"${enabled:=false}"`

	// CertFile and KeyFile are the client certificate/key pair used for mutual
	// TLS. Leave both empty when the server does not require a client cert.
	CertFile string `value:"${cert-file:=}"`
	KeyFile  string `value:"${key-file:=}"`

	// CACertFile is a PEM bundle of root CAs used to verify the server
	// certificate. When empty the host's default root set is used.
	CACertFile string `value:"${ca-cert-file:=}"`
}

// RegistrationConfig binds the instance to advertise, under ${spring.registry}.
// These fields are backend-agnostic: switching from etcd to another registry
// backend is a blank-import swap, not a config migration (starter/DESIGN §3).
type RegistrationConfig struct {
	// ServiceName is the logical name to publish — the same name discovery
	// clients later resolve. Required.
	ServiceName string `value:"${service-name:=}"`

	// Addr is the connectable "host:port" advertised to clients. Required; the
	// starter never guesses it, so a misconfiguration fails at startup.
	Addr string `value:"${addr:=}"`

	// ID overrides the instance id within the service; empty derives a stable one
	// from ServiceName and Addr so restarts replace the same key.
	ID string `value:"${id:=}"`

	// Weight is the load-balancing weight advertised to clients; 0 means the
	// backend default.
	Weight int `value:"${weight:=0}"`

	// Metadata is arbitrary key/value attributes stored with the instance
	// (zone, unit, version, ...), bound from ${spring.registry.metadata.*}.
	Metadata map[string]string `value:"${metadata:=}"`

	// Backend selects which registrar backend to publish to, by the name it was
	// registered under in the stdlib/discovery registrar registry. Defaults to
	// "default", matching EtcdConfig.Name's default.
	Backend string `value:"${backend:=default}"`
}

// ttlSeconds returns the lease TTL in whole seconds, clamped to a minimum of one
// second. etcd leases refuse TTLs below one second.
func (c EtcdConfig) ttlSeconds() int64 {
	d := c.TTL
	if d <= 0 {
		d = 15 * time.Second
	}
	s := int64(d / time.Second)
	if d%time.Second != 0 {
		s++
	}
	if s < 1 {
		s = 1
	}
	return s
}

// buildTLSConfig turns a TLSConfig into a *tls.Config, or nil when TLS is
// disabled. It loads the client key pair and CA bundle from disk when provided.
func buildTLSConfig(c TLSConfig) (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{}
	if c.CertFile != "" || c.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, errutil.Explain(err, "registry-etcd: failed to load TLS key pair")
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if c.CACertFile != "" {
		pem, err := os.ReadFile(c.CACertFile)
		if err != nil {
			return nil, errutil.Explain(err, "registry-etcd: failed to read TLS CA file")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errutil.Explain(nil, "registry-etcd: no certificates found in CA file %s", c.CACertFile)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}
