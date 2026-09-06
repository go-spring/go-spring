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
// ${spring.elasticsearch}.* and the Driver selection key.
package StarterElasticsearch

// Config defines Elasticsearch client connection configuration.
type Config struct {
	// Addresses is the list of Elasticsearch node addresses to connect to,
	// e.g., "http://127.0.0.1:9200". Multiple addresses can be separated by commas.
	Addresses []string `value:"${addresses}" expr:"len($) > 0"`

	// Username is the username for HTTP Basic Authentication, default is empty.
	Username string `value:"${username:=}"`

	// Password is the password for HTTP Basic Authentication, default is empty.
	Password string `value:"${password:=}"`

	// APIKey is the base64-encoded API key for authorization,
	// takes precedence over Username/Password when set. Default is empty.
	APIKey string `value:"${api-key:=}"`

	// ServiceToken is the service account token for authorization, default is empty.
	ServiceToken string `value:"${service-token:=}"`

	// CloudID is the Elastic Cloud deployment ID.
	// When set, it takes precedence over Addresses. Default is empty.
	CloudID string `value:"${cloud-id:=}"`

	// CertificateFingerprint is the SHA256 hex fingerprint of the CA certificate,
	// used to verify self-signed HTTPS endpoints. Default is empty.
	CertificateFingerprint string `value:"${certificate-fingerprint:=}"`

	// MaxRetries is the maximum number of retries for a request, default is 3.
	MaxRetries int `value:"${max-retries:=3}"`

	// DisableRetry disables the retry mechanism entirely, default is false.
	DisableRetry bool `value:"${disable-retry:=false}"`

	// CompressRequestBody enables gzip compression of request bodies, default is false.
	CompressRequestBody bool `value:"${compress-request-body:=false}"`

	// EnableMetrics enables the metrics collection of the transport, default is true.
	EnableMetrics bool `value:"${enable-metrics:=true}"`

	// EnableDebugLogger enables the debug logging of the transport, default is false.
	EnableDebugLogger bool `value:"${enable-debug-logger:=false}"`

	// ServiceName resolves the node addresses through a registered discovery
	// backend instead of the static Addresses list. When set, the endpoints are
	// resolved once at startup and turned into "scheme://host:port" addresses.
	//
	// Limitation: this is a one-shot resolution at startup — the resulting node
	// list is fixed for the client's lifetime. Elasticsearch cluster addresses
	// are typically stable VIPs, so this is usually sufficient; when it is not,
	// leave ServiceName empty and configure Addresses directly. When empty, the
	// static Addresses (or CloudID) are used unchanged.
	ServiceName string `value:"${service-name:=}"`

	// Scheme narrows discovery to endpoints of one transport scheme (e.g. "tls",
	// "https"). Empty (the default) returns every scheme; set it when a service
	// exposes both plain and secure instances and this client should reach only
	// one. Only consulted when ServiceName is set.
	Scheme string `value:"${scheme:=}"`

	// Discovery selects which registered discovery backend resolves ServiceName.
	// It is only consulted when ServiceName is set. A company registers its
	// naming service once via discovery.Register; the default backend name is
	// "default".
	Discovery string `value:"${discovery:=default}"`

	// DiscoveryScheme is the URL scheme ("http" or "https") prepended to each
	// discovered "host:port" endpoint, since discovery yields addresses without a
	// scheme. It is only used when ServiceName is set; default is "http".
	DiscoveryScheme string `value:"${discovery-scheme:=http}"`
}
