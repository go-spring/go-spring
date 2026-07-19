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

package contract

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

// TB is the subset of *testing.T / *testing.B this package needs. Depending on
// the interface rather than the concrete type keeps stdlib free of a testing
// import in its exported API and lets callers pass a fake in their own tests.
type TB interface {
	Helper()
	Cleanup(func())
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Skipf(format string, args ...any)
}

// StubHandler builds an http.Handler that answers each request with the
// response of the first contract whose Request matches. It is the consumer-side
// double: point a declarative HTTP client (go-spring.org/stdlib/httpx) at it and
// the client sees exactly what the provider promised in the contracts.
//
// A request that matches no contract gets 501 Not Implemented with a body that
// lists what was tried, so an out-of-contract call fails loudly instead of
// silently returning a zero value.
func StubHandler(contracts []Contract) http.Handler {
	// Copy so later mutation of the caller's slice cannot change stub behavior.
	cs := append([]Contract(nil), contracts...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		for _, c := range cs {
			if !requestMatches(c, r, body) {
				continue
			}
			for k, v := range c.Response.Headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(c.Response.status())
			_, _ = w.Write(c.Response.Body)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = fmt.Fprintf(w, "no contract matched %s %s\ntried:\n%s",
			r.Method, r.URL.RequestURI(), summarize(cs))
	})
}

// StubServer starts a live httptest.Server backed by StubHandler and registers
// its shutdown with tb.Cleanup, so a test just needs the returned base URL. Use
// it as the Target/base address of the consumer under test.
func StubServer(tb TB, contracts []Contract) *httptest.Server {
	tb.Helper()
	srv := httptest.NewServer(StubHandler(contracts))
	tb.Cleanup(srv.Close)
	return srv
}

// summarize renders the contract request lines for the 501 diagnostic body.
func summarize(cs []Contract) string {
	var b strings.Builder
	for _, c := range cs {
		fmt.Fprintf(&b, "  - %s: %s %s\n", c.Name, c.Request.Method, c.Request.Path)
	}
	return b.String()
}
