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

package StarterCassandra

import (
	"time"

	"go-spring.org/cloud/tlsconf"
)

// Config defines the Cassandra/ScyllaDB client configuration. gocql speaks
// the CQL native protocol, which ScyllaDB also serves, so one starter covers
// both.
type Config struct {
	// Hosts is the initial contact point list, e.g., "127.0.0.1". Entries
	// may carry a port ("host:9042"); the driver discovers the rest of the
	// cluster from these.
	Hosts []string `value:"${hosts}" expr:"len($) > 0"`

	// Keyspace is the default keyspace for the session. Leave empty to
	// connect without one (e.g. to run CREATE KEYSPACE first).
	Keyspace string `value:"${keyspace:=}"`

	// Username is the authenticator user. Both Username and Password must be
	// set together to enable PasswordAuthenticator.
	Username string `value:"${username:=}"`

	// Password is the authenticator password that pairs with Username.
	Password string `value:"${password:=}"`

	// Consistency is the default consistency level: any|one|two|three|
	// quorum|all|local-quorum|each-quorum|local-one. Default
	// "local-quorum".
	Consistency string `value:"${consistency:=local-quorum}"`

	// Timeout bounds query execution on the client side, e.g., "11s".
	Timeout time.Duration `value:"${timeout:=11s}"`

	// ConnectTimeout bounds connection setup, e.g., "11s".
	ConnectTimeout time.Duration `value:"${connect-timeout:=11s}"`

	// CQLVersion is the CQL dialect version (default "3.0.0").
	CQLVersion string `value:"${cql-version:=3.0.0}"`

	// TLS configures client-to-server TLS (tlsconf shared block).
	TLS tlsconf.TLSConfig `value:"${tls}"`
}
