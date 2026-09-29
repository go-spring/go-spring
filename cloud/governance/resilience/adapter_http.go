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

// adapter_http.go is the HTTP client seam: an [http.RoundTripper] that routes
// each request through an [ClientExecutor].

package resilience

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// httpStatusError wraps an HTTP response status that the adapter treated as a
// failure (5xx from an upstream, or a 5xx handler). It implements [Retryable]
// so the executor's retry loop — via [shouldRetry] — retries server errors but
// not client errors, without each adapter reimplementing the classification.
// Callers may still inspect the status through [errors.As].
type httpStatusError struct {
	status    int
	retryable bool // true for 5xx (retry), false for unexpected non-2xx
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("resilience: upstream returned %d", e.status)
}

// Retryable reports whether the error is eligible for retry. Only server errors
// (5xx) are; a 4xx is a definite "no" from the upstream.
func (e *httpStatusError) Retryable() bool { return e.retryable }

// NewRoundTripper wraps base so every request flows through exec. It is the
// HTTP seam of the framework and the widest-coverage adapter: any client built
// on *http.Client (oauth2-client, plain REST clients, ...) gains rate limiting,
// circuit breaking and retry by swapping its Transport, with no change to call
// sites. When exec is nil, base is returned unchanged so wiring stays a no-op
// until a policy is configured — the same zero-config opt-in contract as the
// otelhttp transport.
//
// exec carries the service it protects (see [ClientExecutor]), which is why this seam
// takes no label: the transport is already scoped to one client's upstream, and
// the executor knows which service that is. A caller that needs different
// protection for different upstreams builds one transport — and one executor —
// per upstream.
//
// A 5xx response counts as a failure for the breaker and is eligible for retry
// (only when the request body can be rewound, i.e. Request.GetBody is set, as
// the net/http client arranges for standard body types). Transport errors
// always count as failures. The final response or error is returned to the
// caller unchanged.
func NewRoundTripper(base http.RoundTripper, exec ClientExecutor) http.RoundTripper {
	if exec == nil {
		return base
	}
	if base == nil {
		base = http.DefaultTransport
	}
	return &roundTripper{base: base, exec: exec}
}

type roundTripper struct {
	base http.RoundTripper
	exec ClientExecutor
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	err := rt.exec.Execute(req.Context(), func(ctx context.Context) error {
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
			// to retry; surface the response as a breaker-tripping failure whose
			// Retryable() == true lets the executor's retry loop decide.
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()
			return &httpStatusError{status: r.StatusCode, retryable: true}
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
