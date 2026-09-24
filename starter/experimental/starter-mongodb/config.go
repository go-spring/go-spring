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
// ${spring.mongodb}.*.
package StarterMongoDB

import (
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/security"
)

// Config defines MongoDB client connection configuration.
type Config struct {
	// URI is the MongoDB connection string,
	// e.g., "mongodb://127.0.0.1:27017".
	URI string `value:"${uri}" expr:"$ != ''"`

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

	// TLS configures transport encryption for the connection. It is the shared
	// block from spring/cloud/security; leave TLS.Enabled=false to negotiate no
	// TLS (unless the URI itself requests it).
	TLS security.TLSConfig `value:"${tls}"`

	// Addressing is the shared discovery-citation block: ServiceName resolves
	// the connection address through a discovery backend instead of the URI
	// hosts (mesh mode ignores it — the sidecar owns discovery+LB; it also
	// bypasses MongoDB's own replica-set/mongos topology discovery — the driver
	// dials whatever the naming service hands out), Discovery cites the backend
	// bean (falling back to ${spring.mongodb.default.discovery} via the starter
	// wiring). See discovery.Addressing for the shared contract.
	discovery.Addressing

	// Scheme narrows discovery to endpoints of one transport scheme (e.g. "tls",
	// "https"). Empty (the default) returns every scheme; set it when a service
	// exposes both plain and secure instances and this client should reach only
	// one. Only consulted when ServiceName is set.
	Scheme string `value:"${scheme:=}"`
}
