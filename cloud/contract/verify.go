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
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// Verify replays every contract's request against a live provider at baseURL
// and asserts the provider's answer matches the contract's Response. It is the
// provider-side half of the agreement: a provider that drifts from any contract
// fails here. To exercise a handler in-process without a socket, use
// [VerifyHandler].
//
// Failures are reported per contract with tb.Errorf and do not stop the run, so
// one call surfaces every mismatch at once (assert-style, not fail-fast).
func Verify(tb TB, baseURL string, contracts []Contract) {
	tb.Helper()
	base := strings.TrimRight(baseURL, "/")
	verifyExec(tb, contracts, func(req Request) (int, http.Header, []byte) {
		r := buildRequest(req)
		full, _ := url.Parse(base + r.URL.RequestURI())
		r.URL = full
		r.RequestURI = ""
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			return 0, http.Header{}, []byte(err.Error())
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header, b
	})
}

// VerifyHandler is [Verify] for an in-process http.Handler: the contracts are
// replayed through httptest instead of a socket, which is faster and needs no
// port. A live server (e.g. one started by the app's own framework) is better
// covered by Verify against its real base URL.
func VerifyHandler(tb TB, h http.Handler, contracts []Contract) {
	tb.Helper()
	verifyExec(tb, contracts, func(req Request) (int, http.Header, []byte) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, buildRequest(req))
		res := rec.Result()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header, b
	})
}

// executor runs one contract request and returns the provider's status, headers
// and body. Both entry points (live URL vs in-process handler) collapse to this
// signature so the assertion loop is identical for both.
type executor func(Request) (int, http.Header, []byte)

// verifyExec asserts every contract against the provider behind exec.
func verifyExec(tb TB, contracts []Contract, exec executor) {
	tb.Helper()
	for _, c := range contracts {
		status, header, body := exec(c.Request)

		if want := c.Response.status(); status != want {
			tb.Errorf("contract %q: status = %d, want %d (body: %s)", c.Name, status, want, body)
		}
		for k, v := range c.Response.Headers {
			if got := header.Get(k); got != v {
				tb.Errorf("contract %q: header %q = %q, want %q", c.Name, k, got, v)
			}
		}
		if !bodyEqual(c.Response.Body, body) {
			tb.Errorf("contract %q: body = %s, want %s", c.Name, body, c.Response.Body)
		}
	}
}

// buildRequest materializes a contract Request into an *http.Request with the
// declared query, headers and body applied.
func buildRequest(req Request) *http.Request {
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	r := httptest.NewRequest(method, req.Path, body)
	r.RequestURI = ""
	if len(req.Query) > 0 {
		q := r.URL.Query()
		for k, v := range req.Query {
			q.Set(k, v)
		}
		r.URL.RawQuery = q.Encode()
	}
	for k, v := range req.Headers {
		r.Header.Set(k, v)
	}
	return r
}
