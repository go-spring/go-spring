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
	"context"
	"net/http"

	"go-spring.org/spring/gs"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func init() {
	// Register OAuth2 token sources alongside the HTTP clients over the same
	// "${spring.oauth2.client}" configuration group. Beans are keyed by type
	// plus name, so an oauth2.TokenSource coexists with the *http.Client of
	// the same name. This exposes the raw bearer token for callers that need
	// to inject it themselves (e.g., gRPC metadata) rather than send it via
	// an *http.Client. Token sources hold no closable resource, so no destroy
	// callback is needed.
	gs.Group("${spring.oauth2.client}", newTokenSource, nil)
}

// newTokenSource builds an oauth2.TokenSource that mints and refreshes bearer
// tokens via the client-credentials grant. The returned source caches the
// current token and refreshes it automatically once expired.
func newTokenSource(c Config) (oauth2.TokenSource, error) {
	cfg := &clientcredentials.Config{
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		TokenURL:       c.TokenURL,
		Scopes:         c.Scopes,
		AuthStyle:      c.authStyle(),
		EndpointParams: c.endpointParams(),
	}

	ctx := context.Background()
	if c.Timeout > 0 {
		// oauth2 uses the client stored under oauth2.HTTPClient for the token
		// fetch, so honor Timeout there too.
		ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: c.Timeout})
	}

	return cfg.TokenSource(ctx), nil
}
