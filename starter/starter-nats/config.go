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

package StarterNats

import (
	"time"

	"go-spring.org/stdlib/resilience"
)

// Config defines NATS client connection configuration.
type Config struct {
	// URL is the NATS server URL, e.g., "nats://127.0.0.1:4222".
	// Multiple servers may be comma-separated.
	URL string `value:"${url}"`

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
	TLS TLSConfig `value:"${tls}"`

	// MaxReconnects is the maximum number of reconnect attempts,
	// -1 means unlimited, default is 60.
	MaxReconnects int `value:"${max-reconnects:=60}"`

	// ReconnectWait is the delay between reconnect attempts, default is "2s".
	ReconnectWait time.Duration `value:"${reconnect-wait:=2s}"`

	// ConnectTimeout bounds how long the initial dial waits, default is "5s".
	ConnectTimeout time.Duration `value:"${connect-timeout:=5s}"`

	// JetStream configures the JetStream context derived from this connection.
	JetStream JetStreamConfig `value:"${jetstream}"`

	// Resilience optionally protects outbound calls with rate limiting and
	// circuit breaking. It is disabled by default; when enabled the opt-in
	// PublishGuarded/RequestGuarded methods on Conn route the outbound call
	// through the selected resilience driver. nats has no reject-capable
	// middleware seam, so plain Publish/Request stay unchanged — callers pick
	// per-call whether they want the guard.
	Resilience ResilienceConfig `value:"${resilience:=}"`
}

// ResilienceConfig binds the backend-neutral resilience knobs exposed by
// stdlib/resilience. Driver selects which registered backend enforces them:
// "default" (bundled, zero-dependency) or "sentinel" (recommended, enabled by
// blank-importing starter-resilience). Switching backends is a one-line config
// change — no code touches the guard seam.
type ResilienceConfig struct {
	// Enabled attaches the resilience executor to the connection. When false
	// the guarded methods degrade to plain Publish/Request.
	Enabled bool `value:"${enabled:=false}"`

	// Driver names the registered resilience backend to use.
	Driver string `value:"${driver:=default}"`

	// RateLimit caps sustained throughput in ops per second (0 disables).
	RateLimit float64 `value:"${rate-limit:=0}"`

	// Burst is the momentary allowance above RateLimit (0 = driver default).
	Burst int `value:"${burst:=0}"`

	// ErrorThreshold is the consecutive-failure count that trips the breaker
	// open (0 disables circuit breaking).
	ErrorThreshold int `value:"${error-threshold:=0}"`

	// OpenDuration is how long the breaker stays open before a trial call.
	OpenDuration time.Duration `value:"${open-duration:=0}"`

	// MaxRetries is the number of extra attempts after a failure. Keep 0 for
	// publishing — retrying a publish can duplicate a message; leave retry to
	// the caller who knows whether the message is idempotent.
	MaxRetries int `value:"${max-retries:=0}"`

	// AttemptTimeout bounds each individual attempt (0 = no per-attempt bound).
	AttemptTimeout time.Duration `value:"${attempt-timeout:=0}"`
}

// policy maps the bound config onto the backend-neutral resilience.Policy.
func (r ResilienceConfig) policy() resilience.Policy {
	return resilience.Policy{
		RateLimit:      r.RateLimit,
		Burst:          r.Burst,
		ErrorThreshold: r.ErrorThreshold,
		OpenDuration:   r.OpenDuration,
		MaxRetries:     r.MaxRetries,
		Timeout:        r.AttemptTimeout,
	}
}

// TLSConfig configures transport security for the NATS connection. When Enabled
// is true the client negotiates TLS; the remaining fields are optional and only
// consulted while Enabled is true.
type TLSConfig struct {
	// Enabled turns on TLS for the connection, default is false.
	Enabled bool `value:"${enabled:=false}"`

	// CAFile is the path to a PEM CA bundle used to verify the server
	// certificate. When empty the system root pool is used.
	CAFile string `value:"${ca-file:=}"`

	// CertFile and KeyFile are the client certificate and key used for
	// mutual TLS. Both must be set together, default is empty.
	CertFile string `value:"${cert-file:=}"`
	KeyFile  string `value:"${key-file:=}"`

	// InsecureSkipVerify disables server certificate verification. It is
	// intended for testing only and must not be used in production.
	InsecureSkipVerify bool `value:"${insecure-skip-verify:=false}"`
}

// JetStreamConfig configures the JetStream context. When Enabled is true a
// JetStream context is created from the connection and exposed on Conn.JetStream;
// otherwise Conn.JetStream is nil.
type JetStreamConfig struct {
	// Enabled turns on the JetStream context for this connection,
	// default is false.
	Enabled bool `value:"${enabled:=false}"`
}
