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
	"time"

	"go-spring.org/spring/cloud/tlsconf"
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

	// TLS configures transport encryption for the connection. It is the shared
	// block from spring/cloud/tlsconf; leave TLS.Enabled=false to negotiate no
	// TLS (unless the URI itself requests it).
	TLS tlsconf.TLSConfig `value:"${tls}"`

	// ServiceName resolves the connection address through a registered discovery
	// backend instead of relying solely on the URI hosts. When set, a LiveDialer
	// is injected as the client's ContextDialer, so every new connection reaches
	// a currently-live instance and address changes take effect without
	// rebuilding the client. When empty, the URI hosts are dialed directly.
	//
	// Note: this bypasses MongoDB's own topology discovery (replica set / mongos)
	// — the driver dials whatever the naming service hands out. Use it when the
	// intent is "reach the service via the company naming service"; keep it empty
	// to let the driver manage replica-set/mongos topology from the URI.
	ServiceName string `value:"${service-name:=}"`

	// Discovery selects which registered discovery backend resolves ServiceName.
	// It is only consulted when ServiceName is set. A company registers its
	// naming service once via discovery.Register; the default backend name is
	// "default".
	Discovery string `value:"${discovery:=default}"`
}
