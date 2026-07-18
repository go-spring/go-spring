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

package StarterPulsar

import "time"

// Config defines Pulsar client configuration.
//
// Fields are intentionally flat because pulsar-client-go's ClientOptions is
// itself flat; grouping them into nested structs would only add ceremony.
type Config struct {
	// URL is the Pulsar service URL,
	// e.g., "pulsar://127.0.0.1:6650".
	URL string `value:"${url}"`

	// OperationTimeout is the timeout for creating producers, subscribing,
	// and looking up topics, e.g., "30s".
	OperationTimeout time.Duration `value:"${operation-timeout:=30s}"`

	// ConnectionTimeout is the timeout for establishing a TCP connection, e.g., "5s".
	ConnectionTimeout time.Duration `value:"${connection-timeout:=5s}"`

	// Token is a JWT authentication token, or a path to a file containing the
	// token when TokenFromFile is true. Leave empty to disable token auth.
	Token string `value:"${token:=}"`

	// TokenFromFile switches Token to be interpreted as a file path.
	TokenFromFile bool `value:"${token-from-file:=false}"`

	// TLSTrustCertsFilePath points to a PEM bundle of trusted CA certificates
	// used to verify the broker.
	TLSTrustCertsFilePath string `value:"${tls-trust-certs-file:=}"`

	// TLSCertFile is the client certificate for mTLS. When both TLSCertFile and
	// TLSKeyFile are set, they are wired both as ClientOptions fields and as an
	// Authentication provider via NewAuthenticationTLS.
	TLSCertFile string `value:"${tls-cert-file:=}"`

	// TLSKeyFile is the client private key that pairs with TLSCertFile.
	TLSKeyFile string `value:"${tls-key-file:=}"`

	// TLSAllowInsecure disables server certificate verification. Do not enable
	// in production.
	TLSAllowInsecure bool `value:"${tls-allow-insecure:=false}"`

	// TLSValidateHostname makes the client verify the broker hostname against
	// the certificate. Default is false to preserve pulsar-client-go's default.
	TLSValidateHostname bool `value:"${tls-validate-hostname:=false}"`

	// FailFast enables a startup broker probe. When true, a lookup is issued
	// against HealthCheckTopic right after the client is built so bad broker
	// addresses / auth / TLS surface immediately instead of on first use.
	FailFast bool `value:"${fail-fast:=true}"`

	// HealthCheckTopic is the topic used for the startup lookup probe. A
	// lookup against a non-existent, non-partitioned topic normally succeeds
	// (returns the topic name), so the default is safe on a fresh cluster.
	HealthCheckTopic string `value:"${health-check-topic:=persistent://public/default/__health_check}"`
}
