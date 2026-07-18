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
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// Config defines an OAuth2 client-credentials configuration. Each instance
// yields an *http.Client that transparently fetches and refreshes a bearer
// token before every outbound request, so callers use it like an ordinary
// HTTP client when talking to a protected downstream service.
type Config struct {
	// ClientID is the OAuth2 client identifier issued by the authorization server.
	ClientID string `value:"${client-id}"`

	// ClientSecret is the OAuth2 client secret issued by the authorization server.
	ClientSecret string `value:"${client-secret}"`

	// TokenURL is the token endpoint that mints access tokens,
	// e.g., "https://auth.example.com/oauth/token".
	TokenURL string `value:"${token-url}"`

	// Scopes is the optional list of scopes requested with the token.
	Scopes []string `value:"${scopes:=}"`

	// EndpointParams carries extra parameters sent to the token endpoint,
	// such as an "audience" (Auth0) or "resource" (Azure AD). Optional.
	EndpointParams map[string]string `value:"${endpoint-params:=}"`

	// AuthStyle selects how client credentials are sent to the token endpoint:
	//   "auto"   — let the library probe (default),
	//   "header" — HTTP Basic auth header,
	//   "params" — request body parameters.
	AuthStyle string `value:"${auth-style:=auto}"`

	// Timeout bounds each HTTP request made by the resulting client,
	// including the token fetch. 0 means no timeout, e.g., "5s".
	Timeout time.Duration `value:"${timeout:=0}"`
}

// authStyle maps the configured auth-style string to an oauth2.AuthStyle.
func (c Config) authStyle() oauth2.AuthStyle {
	switch c.AuthStyle {
	case "header":
		return oauth2.AuthStyleInHeader
	case "params":
		return oauth2.AuthStyleInParams
	default:
		return oauth2.AuthStyleAutoDetect
	}
}

// endpointParams converts EndpointParams into the url.Values form expected by
// clientcredentials.Config. It returns nil when no extra params are configured.
func (c Config) endpointParams() url.Values {
	if len(c.EndpointParams) == 0 {
		return nil
	}
	v := make(url.Values, len(c.EndpointParams))
	for k, val := range c.EndpointParams {
		v.Set(k, val)
	}
	return v
}
