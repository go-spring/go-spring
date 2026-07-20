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

package StarterOAuth2Client

import (
	"io"
	"net/http"

	"go-spring.org/spring/gs"
	"go-spring.org/spring/resilience"
	"golang.org/x/oauth2/clientcredentials"
)

func init() {
	// Register multiple OAuth2 client-credentials HTTP clients as a group.
	// Each instance is created from the configuration under "${spring.oauth2.client}",
	// allowing several downstream services (each with its own credentials) to be
	// defined dynamically. The resulting *http.Client caches and refreshes tokens
	// internally; when resilience is enabled its transport owns a closable
	// executor, so a destroy hook releases it (a no-op otherwise).
	gs.Group("${spring.oauth2.client}", newClient, destroyClient)
}

// newClient builds an *http.Client whose transport injects an OAuth2 bearer
// token obtained via the client-credentials grant. Tokens are fetched lazily on
// the first request and refreshed automatically once expired. Both the token
// exchange and downstream requests are traced via otelContext (no-op without
// starter-otel). When resilience is enabled the transport is additionally
// wrapped so downstream requests flow through the configured rate limiter,
// circuit breaker and retry — the bearer token is already attached before the
// resilience layer runs, so each protected attempt is a complete request.
func newClient(c Config) (*http.Client, error) {
	cfg := &clientcredentials.Config{
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		TokenURL:       c.TokenURL,
		Scopes:         c.Scopes,
		AuthStyle:      c.authStyle(),
		EndpointParams: c.endpointParams(),
	}

	client := cfg.Client(otelContext(c.Timeout))
	if c.Timeout > 0 {
		client.Timeout = c.Timeout
	}

	if c.Resilience.Enabled {
		drv, err := resilience.MustGetDriver(c.Resilience.Driver)
		if err != nil {
			return nil, err
		}
		exec, err := drv.NewExecutor(c.Resilience.policy())
		if err != nil {
			return nil, err
		}
		client.Transport = resilience.NewRoundTripper(client.Transport, exec, nil)
	}
	return client, nil
}

// destroyClient releases the resilience executor behind the client's transport,
// if any. Plain (non-resilience) clients hold no closable resource, so the
// type-assertion simply fails and the hook does nothing.
func destroyClient(client *http.Client) error {
	if c, ok := client.Transport.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
