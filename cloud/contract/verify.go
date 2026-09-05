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

// VerifyURL replays every contract's request against a live provider at baseURL
// and asserts the provider's answer matches the contract's Response. It is the
// provider-side half of the agreement: a provider that drifts from any contract
// fails here. To exercise a handler in-process without a socket, use
// [VerifyHandler].
//
// Failures are reported per contract with tb.Errorf and do not stop the run, so
// one call surfaces every mismatch at once (assert-style, not fail-fast).
func VerifyURL(tb TB, baseURL string, contracts []Contract) {
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

// VerifyHandler is [VerifyURL] for an in-process http.Handler: the contracts are
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
	for i := range contracts {
		if err := contracts[i].validate(); err != nil {
			tb.Fatalf("%s", err)
		}
	}
	for _, c := range contracts {
		status, header, body := exec(c.Request)

		if want := c.Response.Status; status != want {
			tb.Errorf("contract %q: status = %d, want %d (body: %s)", c.Name, status, want, body)
		}
		for k, want := range c.Response.Headers {
			if !valuesEqual(want, header.Values(k)) {
				tb.Errorf("contract %q: header %q = %q, want %q", c.Name, k, header.Values(k), want)
			}
		}
		ct := header.Get("Content-Type")
		if ct == "" {
			// The provider sent no Content-Type; the contract's own declared
			// header is the next best statement of the body's format.
			ct = c.Response.Headers.Get("Content-Type")
		}
		if !bodyEqual(c.Response.Body, body, ct) {
			tb.Errorf("contract %q: body = %s, want %s", c.Name, body, c.Response.Body)
		}
	}
}

// buildRequest materializes a contract Request into an *http.Request with the
// declared query, headers and body applied.
func buildRequest(req Request) *http.Request {
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	r := httptest.NewRequest(req.Method, req.Path, body)
	r.RequestURI = ""
	if len(req.Query) > 0 {
		q := r.URL.Query()
		for k, vs := range req.Query {
			q[k] = vs
		}
		r.URL.RawQuery = q.Encode()
	}
	for k, vs := range req.Headers {
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}
	return r
}
