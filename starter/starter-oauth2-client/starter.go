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
	"net/http"

	"go-spring.org/spring/gs"
	"golang.org/x/oauth2/clientcredentials"
)

func init() {
	// Register multiple OAuth2 client-credentials HTTP clients as a group.
	// Each instance is created from the configuration under "${spring.oauth2.client}",
	// allowing several downstream services (each with its own credentials) to be
	// defined dynamically. The resulting *http.Client caches and refreshes tokens
	// internally and holds no closable resource, so no destroy callback is needed.
	gs.Group("${spring.oauth2.client}", newClient, nil)
}

// newClient builds an *http.Client whose transport injects an OAuth2 bearer
// token obtained via the client-credentials grant. Tokens are fetched lazily on
// the first request and refreshed automatically once expired. Both the token
// exchange and downstream requests are traced via otelContext (no-op without
// starter-otel).
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
	return client, nil
}
