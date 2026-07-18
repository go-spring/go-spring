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

package StarterRabbitMQ

import "time"

// Config defines RabbitMQ connection configuration.
type Config struct {
	// URL is the AMQP connection URL,
	// e.g., "amqp://guest:guest@127.0.0.1:5672/".
	// Use the "amqps://" scheme (or set TLS.Enabled) to negotiate TLS.
	URL string `value:"${url}"`

	// Vhost overrides the virtual host parsed from the URL, default is empty.
	Vhost string `value:"${vhost:=}"`

	// Heartbeat is the interval for connection heartbeats,
	// 0 uses the URL/server default, e.g., "10s".
	Heartbeat time.Duration `value:"${heartbeat:=10s}"`

	// TLS configures transport encryption. It is activated either explicitly by
	// Enabled=true or implicitly when the URL uses the "amqps://" scheme; the
	// certificate files are optional and only needed for a custom CA or mTLS.
	TLS TLSConfig `value:"${tls}"`
}

// TLSConfig configures TLS for AMQPS. Naming aligns with the franz-go based
// starter-kafka's TLSConfig so operators can reuse mental models; ServerName is
// added because AMQPS deployments often terminate on a hostname that differs
// from the URL host (e.g., LB-fronted brokers).
type TLSConfig struct {
	// Enabled forces TLS on regardless of the URL scheme, default is false.
	// When the URL already uses "amqps://" TLS is on even if Enabled is false.
	Enabled bool `value:"${enabled:=false}"`

	// CACert is the path to a PEM CA bundle used to verify the broker
	// certificate; empty uses the system roots.
	CACert string `value:"${ca-cert:=}"`

	// ClientCert and ClientKey are the PEM client certificate/key pair for
	// mutual TLS; both empty disables client authentication.
	ClientCert string `value:"${client-cert:=}"`
	ClientKey  string `value:"${client-key:=}"`

	// InsecureSkipVerify disables broker certificate verification. Never
	// enable it outside development, default is false.
	InsecureSkipVerify bool `value:"${insecure-skip-verify:=false}"`

	// ServerName overrides the SNI/verification hostname sent to the broker;
	// empty falls back to the host parsed from the URL.
	ServerName string `value:"${server-name:=}"`
}
