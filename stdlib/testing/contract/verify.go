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

// Verify replays every contract's request against the provider and asserts the
// provider's answer matches the contract's Response. It is the provider-side
// half of the agreement: a provider that drifts from any contract fails here.
//
// target is either a base URL string ("http://127.0.0.1:8080", to hit a running
// server) or an http.Handler (to exercise the mux in-process without a socket).
// Failures are reported per contract with tb.Errorf and do not stop the run, so
// one Verify call surfaces every mismatch at once (assert-style, not fail-fast).
func Verify(tb TB, target any, contracts []Contract) {
	tb.Helper()

	exec, err := executorFor(target)
	if err != nil {
		tb.Fatalf("contract: verify: %v", err)
		return
	}

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

// executor runs one contract request and returns the provider's status, headers
// and body. The two targets (live URL vs in-process handler) collapse to this
// signature so the assertion loop above is identical for both.
type executor func(Request) (int, http.Header, []byte)

// executorFor builds an executor for a base URL string or an http.Handler.
func executorFor(target any) (executor, error) {
	switch t := target.(type) {
	case string:
		base := strings.TrimRight(t, "/")
		return func(req Request) (int, http.Header, []byte) {
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
		}, nil
	case http.Handler:
		return func(req Request) (int, http.Header, []byte) {
			rec := httptest.NewRecorder()
			t.ServeHTTP(rec, buildRequest(req))
			res := rec.Result()
			b, _ := io.ReadAll(res.Body)
			return res.StatusCode, res.Header, b
		}, nil
	default:
		return nil, &badTargetError{}
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

// badTargetError reports an unsupported Verify target type.
type badTargetError struct{}

func (*badTargetError) Error() string {
	return "target must be a base URL string or an http.Handler"
}
