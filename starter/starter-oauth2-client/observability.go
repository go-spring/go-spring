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
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
)

// otelContext returns a context carrying an *http.Client whose transport is
// wrapped with otelhttp, so outbound requests emit client spans through the OTel
// globals that starter-otel installs. When starter-otel is absent those globals
// are no-ops, so this stays a zero-config opt-in that adds no spans and rewrites
// no bytes — the same contract as go-redis's redisotel hooks.
//
// A single instrumentation point covers both call paths without double counting:
//
//   - oauth2 reads this client from the context (oauth2.HTTPClient) for the
//     token-endpoint exchange, so token fetch/refresh requests are traced.
//   - clientcredentials.Config.Client reuses this client's transport as the Base
//     of the oauth2.Transport it returns; the auth header is added before Base
//     runs, so each downstream business request produces exactly one span that
//     already carries the bearer token.
func otelContext(timeout time.Duration) context.Context {
	base := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	if timeout > 0 {
		base.Timeout = timeout
	}
	return context.WithValue(context.Background(), oauth2.HTTPClient, base)
}
