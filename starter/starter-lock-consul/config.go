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

package StarterLockConsul

import (
	"time"

	"go-spring.org/spring/starter"
)

// Config binds one Consul-backed distributed-lock instance under
// "spring.lock.<name>". Every instance owns its own consul API client; two
// instances that need to share a client should not exist — declare one entry
// and inject it by name.
type Config struct {
	// Address is the Consul agent endpoint, e.g. "127.0.0.1:8500". Required;
	// the starter fails fast at startup when it is empty rather than silently
	// defaulting to localhost.
	Address string `value:"${address}"`

	// Scheme selects the transport, "http" or "https". TLS actually kicks in
	// through the nested TLS block below; setting Scheme=https on its own
	// only chooses the URL scheme.
	Scheme string `value:"${scheme:=http}"`

	// Token is the Consul ACL token used by the API client. It is distinct
	// from the per-acquisition fencing token exposed on the Lock handle.
	Token string `value:"${token:=}"`

	// TTL is the session TTL that Consul auto-renews behind api.Lock. Consul
	// requires it in the [10s, 86400s] range; values outside that range are
	// clamped at startup. Defaults to 30s.
	TTL time.Duration `value:"${ttl:=30s}"`

	// KeyPrefix is prepended to every lock key so that many applications can
	// share one Consul cluster without colliding on flat keys. Defaults to
	// "lock/".
	KeyPrefix string `value:"${key-prefix:=lock/}"`

	// TLS configures optional TLS to the Consul agent. It is applied only
	// when TLS.Enabled is true; otherwise the client dials in plaintext.
	// TLS.ServerName overrides the SNI/hostname checked against the server
	// certificate when dialing by IP.
	TLS starter.TLSConfig `value:"${tls}"`
}
