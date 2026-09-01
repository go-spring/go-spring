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

	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
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
	gs.Module(gs.OnProperty("spring.oauth2.client"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.oauth2.client}", func(name string, c Config) error {
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
			).Name(name).Destroy(destroyClient).Caller(1)
			return nil
		})
	})
}

// newClient builds an *http.Client whose transport injects an OAuth2 bearer
// token obtained via the client-credentials grant. Tokens are fetched lazily on
// the first request and refreshed automatically once expired. Both the token
// exchange and downstream requests are traced via otelContext (no-op without
// starter-otel). The transport is additionally wrapped so downstream requests
// flow through the resilience executor (rate limiter, circuit breaker, retry)
// resolved via [resilience.ExecutorFor] — a transparent no-op when governance
// is off — so the bearer token is already attached before the resilience layer
// runs and each protected attempt is a complete request.
func newClient(ctx *gs.ContextProvider, name string, c Config) (*http.Client, error) {

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

	// Resolve the resilience executor through the NEUTRAL provider seam
	// [resilience.ExecutorFor]: starter-govern registers a provider backed by the
	// governance center, so this client gets its rate-limit/breaker/retry policy
	// WITHOUT injecting *governance.Center or even importing cloud/governance. When
	// governance is not configured the seam yields a transparent no-op executor,
	// so this call is always safe. Always non-nil; resolution is deferred to call
	// time, so the order of this setup relative to starter-govern is irrelevant.
	resource := resilience.ResourceLabel("oauth2", c.ClientID)
	exec := resilience.ExecutorFor(resource)
	// Wrap so breaker trips / rejects / retries emit span + counter +
	// histogram + access log (the resilience core emits none). nil-safe,
	// no-op without starter-otel.
	exec = resilience.WrapExecutor(exec, "oauth2", observe.ObserveConfig{})
	// Scope the roundtripper's per-call Execute to the same label so limiter/
	// breaker state all agree.
	client.Transport = resilience.NewRoundTripper(client.Transport, exec, func(*http.Request) string { return resource })
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
