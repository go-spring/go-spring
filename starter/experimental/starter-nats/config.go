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
// ${spring.nats}.* and the nested JetStreamConfig.
package StarterNats

import (
	"time"

	"go-spring.org/cloud/tlsconf"
)

// Config defines NATS client connection configuration.
type Config struct {
	// URL is the NATS server URL, e.g., "nats://127.0.0.1:4222".
	// Multiple servers may be comma-separated.
	URL string `value:"${url}" expr:"$ != ''"`

	// Name is the connection name reported to the server, default is empty.
	Name string `value:"${name:=}"`

	// Username is the auth username, default is empty.
	Username string `value:"${username:=}"`

	// Password is the auth password, default is empty.
	Password string `value:"${password:=}"`

	// Token is the auth token, an alternative to username/password,
	// default is empty.
	Token string `value:"${token:=}"`

	// CredsFile is the path to a NATS credentials file (JWT + nkey seed),
	// used for decentralized (NATS 2.x / NGS) auth. Default is empty.
	CredsFile string `value:"${creds-file:=}"`

	// NKeyFile is the path to an nkey seed file used for nkey auth,
	// an alternative to CredsFile. Default is empty.
	NKeyFile string `value:"${nkey-file:=}"`

	// TLS configures the transport security for the connection.
	TLS tlsconf.TLSConfig `value:"${tls}"`

	// MaxReconnects is the maximum number of reconnect attempts,
	// -1 means unlimited, default is 60.
	MaxReconnects int `value:"${max-reconnects:=60}"`

	// ReconnectWait is the delay between reconnect attempts, default is "2s".
	ReconnectWait time.Duration `value:"${reconnect-wait:=2s}"`

	// ConnectTimeout bounds how long the initial dial waits, default is "5s".
	ConnectTimeout time.Duration `value:"${connect-timeout:=5s}"`

	// JetStream configures the JetStream context derived from this connection.
	JetStream JetStreamConfig `value:"${jetstream}"`

	// Driver specifies which NATS driver to use, defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}

// JetStream context is created from the connection and exposed on Conn.JetStream;
// otherwise Conn.JetStream is nil.
type JetStreamConfig struct {
	// Enabled turns on the JetStream context for this connection,
	// default is false.
	Enabled bool `value:"${enabled:=false}"`
}
