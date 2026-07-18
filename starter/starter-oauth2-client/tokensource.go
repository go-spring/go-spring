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
	"sync/atomic"
	"time"

	"go-spring.org/spring/gs"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func init() {
	// Register OAuth2 token sources alongside the HTTP clients over the same
	// "${spring.oauth2.client}" configuration group. Beans are keyed by type
	// plus name, so a *TokenSource coexists with the *http.Client of the same
	// name. It exposes the raw bearer token for callers that need to inject it
	// themselves (e.g., gRPC metadata) rather than send it via an *http.Client,
	// and additionally surfaces the cached token's status for observability.
	// Token sources hold no closable resource, so no destroy callback is needed.
	gs.Group("${spring.oauth2.client}", newTokenSource, nil)
}

// TokenSource wraps an oauth2.TokenSource (client-credentials grant) and records
// the most recently minted token so callers can observe the current bearer
// token, its validity, and its expiry without forcing a fetch. It satisfies
// oauth2.TokenSource, so it drops into anything expecting one.
type TokenSource struct {
	src  oauth2.TokenSource
	last atomic.Pointer[oauth2.Token]
}

// Token returns a valid token, fetching or refreshing it as needed, and caches
// the result for later observation via Peek/Valid/Expiry.
func (t *TokenSource) Token() (*oauth2.Token, error) {
	tok, err := t.src.Token()
	if err == nil {
		t.last.Store(tok)
	}
	return tok, err
}

// Peek returns the most recently observed token without triggering a fetch, or
// nil if Token has not yet been called.
func (t *TokenSource) Peek() *oauth2.Token {
	return t.last.Load()
}

// Valid reports whether a token has been fetched and is not expired. It does not
// trigger a fetch.
func (t *TokenSource) Valid() bool {
	tok := t.last.Load()
	return tok != nil && tok.Valid()
}

// Expiry returns the expiry of the most recently observed token, or the zero
// time if none has been fetched.
func (t *TokenSource) Expiry() time.Time {
	if tok := t.last.Load(); tok != nil {
		return tok.Expiry
	}
	return time.Time{}
}

// newTokenSource builds a *TokenSource over an oauth2.TokenSource that mints and
// refreshes bearer tokens via the client-credentials grant. The underlying
// source caches the current token and refreshes it automatically once expired.
func newTokenSource(c Config) (*TokenSource, error) {
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

	return &TokenSource{src: cfg.TokenSource(ctx)}, nil
}
