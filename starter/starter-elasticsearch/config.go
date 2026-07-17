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

package StarterElasticsearch

import (
	"github.com/elastic/go-elasticsearch/v8"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Config defines Elasticsearch client connection configuration.
type Config struct {
	// Addresses is the list of Elasticsearch node addresses to connect to,
	// e.g., "http://127.0.0.1:9200". Multiple addresses can be separated by commas.
	Addresses []string `value:"${addresses}"`

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

	// EnableMetrics enables the metrics collection of the transport, default is false.
	EnableMetrics bool `value:"${enable-metrics:=false}"`

	// EnableDebugLogger enables the debug logging of the transport, default is false.
	EnableDebugLogger bool `value:"${enable-debug-logger:=false}"`

	// Driver specifies which Elasticsearch driver to use, defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}

// Driver interface defines how to create an Elasticsearch client.
type Driver interface {
	CreateClient(c Config) (*elasticsearch.Client, error)
}

// RegisterDriver registers an Elasticsearch driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("elasticsearch driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Elasticsearch client based on the provided configuration.
func (DefaultDriver) CreateClient(c Config) (*elasticsearch.Client, error) {
	return elasticsearch.NewClient(elasticsearch.Config{
		Addresses:              c.Addresses,
		Username:               c.Username,
		Password:               c.Password,
		APIKey:                 c.APIKey,
		ServiceToken:           c.ServiceToken,
		CloudID:                c.CloudID,
		CertificateFingerprint: c.CertificateFingerprint,
		MaxRetries:             c.MaxRetries,
		DisableRetry:           c.DisableRetry,
		CompressRequestBody:    c.CompressRequestBody,
		EnableMetrics:          c.EnableMetrics,
		EnableDebugLogger:      c.EnableDebugLogger,
	})
}
