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

	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
	"golang.org/x/oauth2/clientcredentials"
)

func init() {
	// Register multiple OAuth2 client-credentials HTTP clients as a group.
	// Each instance is created from the configuration under "${spring.oauth2.client}",
	// allowing several downstream services (each with its own credentials) to be
	// defined dynamically. The resulting *http.Client caches and refreshes tokens
	// internally; its transport always owns an executor (a transparent no-op when
	// governance is off), so a destroy hook releases it uniformly.
	//
	// A gs.Module (rather than gs.Group) is used so each instance's bean can be
	// paired with a Name + Destroy hook carrying the call-site file:line for
	// diagnostics.
	gs.Module(gs.OnProperty("spring.oauth2.client.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.oauth2.client.instances}", func(name string, c Config) error {
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				// The governance beans are REQUIRED: each is registered by the package that
				// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
				// starter imports — "governance off" is spring.governance.enabled=false, never
				// an absent bean.
				gs.IndexArg(3, gs.TagArg("")), // *resilience.Manager
			).Name(name).Destroy(destroyClient).Caller(1)
			return nil
		})
	})
}

// The authorization_code configurations (see authcode.go).
func init() {
	// Register multiple OAuth2 authorization_code configurations as a group.
	// Each instance is created from the configuration under "${spring.oauth2.authcode}".
	// The resulting *oauth2.Config holds no closable resource, so no destroy callback is needed.
	gs.Group("${spring.oauth2.authcode.instances}", newAuthCodeConfig, nil)
}

// The token sources over the same instance group (see tokensource.go).
func init() {
	// Register OAuth2 token sources alongside the HTTP clients over the same
	// "${spring.oauth2.client}" configuration group. Beans are keyed by type
	// plus name, so a *TokenSource coexists with the *http.Client of the same
	// name. It exposes the raw bearer token for callers that need to inject it
	// themselves (e.g., gRPC metadata) rather than send it via an *http.Client,
	// and additionally surfaces the cached token's status for observability.
	// Token sources hold no closable resource, so no destroy callback is needed.
	gs.Group("${spring.oauth2.client.instances}", newTokenSource, nil)
}

// newClient builds an *http.Client whose transport injects an OAuth2 bearer
// token obtained via the client-credentials grant. Tokens are fetched lazily on
// the first request and refreshed automatically once expired. Both the token
// exchange and downstream requests are traced via otelContext (no-op without
// starter-otel). The transport is additionally wrapped so downstream requests
// flow through the resilience executor (rate limiter, circuit breaker, retry)
// built from the injected [resilience.Manager] — a transparent no-op when
// governance is off — so the bearer token is already attached before the
// resilience layer runs and each protected attempt is a complete request.
//
// mgr is the governance bean gs injects (nil in a standalone call); a nil
// manager is normalized to an unarmed one, whose executor is a transparent
// pass-through, so governance off and standalone callers behave identically.
func newClient(ctx *gs.ContextProvider, name string, c Config, mgr *resilience.Manager) (*http.Client, error) {

	cfg := &clientcredentials.Config{
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		TokenURL:       c.TokenURL,
		Scopes:         c.Scopes,
		AuthStyle:      c.authStyle(),
		EndpointParams: c.endpointParams(),
	}

	log.Debugf(ctx.Context, log.TagAppDef, "creating oauth2 client clientID=%s tokenURL=%s timeout=%s", c.ClientID, c.TokenURL, c.Timeout)

	client := cfg.Client(otelContext(c.Timeout))
	if c.Timeout > 0 {
		client.Timeout = c.Timeout
	}

	// Build the resilience executor from the injected manager, so this client gets
	// its rate-limit/breaker/retry policy from the governance document without
	// naming *governance.Center. An unarmed manager yields a transparent
	// pass-through executor, so this call is always safe. The executor handle
	// resolves its backing implementation per call, so the order of this setup
	// relative to the container's wiring is irrelevant.
	service := resilience.ServiceLabel("oauth2", c.ClientID)
	exec := mgr.ClientExecutorFor("oauth2", service)
	client.Transport = resilience.NewRoundTripper(client.Transport, exec)
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
