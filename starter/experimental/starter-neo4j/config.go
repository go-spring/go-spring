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
	"time"

	"go-spring.org/cloud/tlsconf"
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
	TLS tlsconf.TLSConfig `value:"${tls}"`

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

	// Scheme narrows discovery to endpoints of one transport scheme (e.g. "tls",
	// "https"). Empty (the default) returns every scheme; set it when a service
	// exposes both plain and secure instances and this client should reach only
	// one. Only consulted when ServiceName is set.
	Scheme string `value:"${scheme:=}"`

	// Discovery names the discovery backend bean that resolves ServiceName.
	// It is only consulted when ServiceName is set. The starter wiring resolves
	// this label to the backend bean and hands the result to the client assembly
	// (and on to the Driver) as the backend argument. Empty means unset: no
	// discovery backend is wired and the entry must not route by service-name
	// alone.
	Discovery string `value:"${discovery:=}"`
}
