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

package resilience

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// ResourceFunc derives the resilience resource key from a request. Requests to
// different resources get independent limiter and breaker state. The default
// keys on the request host, isolating protection per downstream service.
type ResourceFunc func(*http.Request) string

// NewRoundTripper wraps base so every request flows through exec. It is the
// HTTP seam of the framework and the widest-coverage adapter: any client built
// on *http.Client (oauth2-client, plain REST clients, ...) gains rate limiting,
// circuit breaking and retry by swapping its Transport, with no change to call
// sites. When exec is nil, base is returned unchanged so wiring stays a no-op
// until a policy is configured — the same zero-config opt-in contract as the
// otelhttp transport.
//
// A 5xx response counts as a failure for the breaker and is eligible for retry
// (only when the request body can be rewound, i.e. Request.GetBody is set, as
// the net/http client arranges for standard body types). Transport errors
// always count as failures. The final response or error is returned to the
// caller unchanged.
func NewRoundTripper(base http.RoundTripper, exec Executor, resource ResourceFunc) http.RoundTripper {
	if exec == nil {
		return base
	}
	if base == nil {
		base = http.DefaultTransport
	}
	if resource == nil {
		resource = func(r *http.Request) string { return r.URL.Host }
	}
	return &roundTripper{base: base, exec: exec, resource: resource}
}

type roundTripper struct {
	base     http.RoundTripper
	exec     Executor
	resource ResourceFunc
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	err := rt.exec.Execute(req.Context(), rt.resource(req), func(ctx context.Context) error {
		// Rewind the body for each attempt so retries send the full payload;
		// requests without a rewindable body simply run once (the executor's
		// retry loop stops on the first success).
		attempt := req.Clone(ctx)
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return err
			}
			attempt.Body = body
		}
		r, err := rt.base.RoundTrip(attempt)
		if err != nil {
			return err
		}
		if r.StatusCode >= 500 {
			// Drain and close so the connection can be reused before we decide
			// to retry; surface the response as a breaker-tripping failure.
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			return fmt.Errorf("resilience: upstream returned %d", r.StatusCode)
		}
		resp = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Close lets an owner (e.g. a starter's destroy hook) release the underlying
// executor by type-asserting the transport to io.Closer.
func (rt *roundTripper) Close() error { return rt.exec.Close() }
