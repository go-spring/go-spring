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

// config.go is the config concept: the per-instance Config bound under
// ${spring.neo4j}.*.

package StarterNeo4j

import (
	"net/url"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/security"
	"go-spring.org/stdlib/errutil"
)

// Config defines Neo4j connection configuration.
type Config struct {
	// URI is the Neo4j connection URI, e.g., "neo4j://127.0.0.1:7687" or
	// "bolt://127.0.0.1:7687". The scheme selects routing and encryption.
	URI string `value:"${uri}" expr:"$ != ''"`

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
	// selected by the URI scheme — tls.enabled does NOT control it here, it is
	// a placeholder for config-shape parity; these fields only customize the
	// trust store, peer name and client certificate. They are ignored for the
	// plaintext "bolt"/"neo4j" schemes.
	TLS security.TLSConfig `value:"${tls}"`

	// ServiceName resolves the routing driver's seed address through a registered
	// discovery backend instead of the URI host, and keeps the resolver behind
	// the driver's AddressResolver fallback so a routing client can re-find the
	// cluster. It requires a routing URI scheme ("neo4j", "neo4j+s",
	// "neo4j+ssc") — see [Config.Validate]: with a direct scheme the driver
	// builds one pool over one address and never asks discovery again, so a
	// service name there would silently pin the first resolved address.
	//
	// Discovery here is not client-side load balancing: the driver owns server
	// selection for the lifetime of the pool (routing table + its own
	// least-penalty choice). For a cluster, or a single instance (which exposes
	// a routing table since Neo4j 4.0), that is the right layer. Independent
	// instances published under one name are NOT balanced by the driver; use a
	// VIP/L4 in front of them or join them into a cluster.
	//
	// Addressing is the shared discovery-citation block (see discovery.Addressing
	// for the shared contract); the family-specific caveats above the fields it
	// replaced still apply. Discovery falls back to ${spring.neo4j.default.discovery} via
	// the starter wiring.
	discovery.Addressing

	// Scheme narrows discovery to endpoints of one transport scheme (e.g. "tls",
	// "https"). Empty (the default) returns every scheme; set it when a service
	// exposes both plain and secure instances and this client should reach only
	// one. Only consulted when ServiceName is set.
	Scheme string `value:"${scheme:=}"`

	// Ping enables the startup connectivity probe: when true the constructor
	// pings the backend once and fails startup if it is unreachable, surfacing
	// misconfiguration early. Default is false: a backend that is not up yet
	// must not block the application from starting; connectivity problems
	// surface on first use instead. Set true to restore fail-fast behaviour.
	Ping bool `value:"${ping:=false}"`

	// Health controls whether the starter contributes a health.Indicator bean
	// for this instance (readiness/startup probes via starter-actuator). On by
	// default; set false to keep this instance out of the aggregated health
	// report, e.g. for a backend whose downtime must not pull the pod out of
	// rotation.
	Health bool `value:"${health:=true}"`
}

// Validate enforces the scheme × service-name contract at bind time. Discovery
// keeps following the naming service only through the routing driver's
// AddressResolver, so a service name is meaningful only with a routing URI
// scheme ("neo4j", "neo4j+s", "neo4j+ssc"). A direct scheme ("bolt", "bolt+s",
// "bolt+ssc") pins the one address the driver was built with and never
// consults discovery again; pairing it with a service name would silently make
// the resolved address permanent, so the pair is rejected here rather than
// degraded.
//
// A direct address stays valid — a VIP, a sidecar, or a single instance just
// carries its address in the URI and leaves service-name unset.
func (c Config) Validate() error {
	if c.ServiceName == "" {
		return nil
	}
	u, err := url.Parse(c.URI)
	if err != nil {
		return errutil.Explain(err, "neo4j: parse uri %q", c.URI)
	}
	switch u.Scheme {
	case "neo4j", "neo4j+s", "neo4j+ssc":
		return nil
	}
	return errutil.Explain(nil,
		"neo4j: service-name requires a routing uri scheme (neo4j, neo4j+s, neo4j+ssc), got %q; drop service-name to dial the configured address directly, or switch the scheme",
		u.Scheme)
}
