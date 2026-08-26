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

package StarterHTTPClient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-spring.org/stdlib/httpclt"
	"go-spring.org/stdlib/testing/assert"
)

// echoReq records what arrived and answers with a fixed JSON body.
type echoReq struct {
	Method string              `json:"method"`
	Path   string              `json:"path"`
	Query  string              `json:"query"`
	Body   map[string]any      `json:"body"`
	Header http.Header     `json:"header"`
}

func newEchoServer(t *testing.T) (*httptest.Server, *echoReq) {
	t.Helper()
	var captured echoReq
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = echoReq{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header}
		if len(b) > 0 {
			_ = json.Unmarshal(b, &captured.Body)
		}
		_, _ = w.Write([]byte(`{"ok":true,"echo":"` + r.Method + `"}`))
	}))
	t.Cleanup(svr.Close)
	return svr, &captured
}

// targetOf strips the scheme so the test server's host:port is the route key.
func targetOf(svr *httptest.Server) string {
	return svr.URL[len("http://"):]
}

func TestGetDecodesJSON(t *testing.T) {
	svr, captured := newEchoServer(t)
	type resp struct {
		OK   bool   `json:"ok"`
		Echo string `json:"echo"`
	}
	httpResp, got, err := Get[resp](context.Background(), targetOf(svr), "/orders/1")
	assert.That(t, err).Nil()
	assert.That(t, httpResp.StatusCode).Equal(http.StatusOK)
	assert.That(t, got.OK).True()
	assert.That(t, got.Echo).Equal(http.MethodGet)
	assert.That(t, captured.Method).Equal(http.MethodGet)
	assert.That(t, captured.Path).Equal("/orders/1")
}

func TestPostSendsJSONBodyAndDecodes(t *testing.T) {
	svr, captured := newEchoServer(t)
	type resp struct {
		OK bool `json:"ok"`
	}
	_, got, err := Post[resp](context.Background(), targetOf(svr), "/orders",
		map[string]any{"id": 7, "name": "pen"})
	assert.That(t, err).Nil()
	assert.That(t, got.OK).True()
	assert.That(t, captured.Method).Equal(http.MethodPost)
	// The body arrived as JSON (decoded server-side); jsonflow matches field
	// names strictly, hence the explicit json tags on the test types.
	assert.That(t, captured.Body["id"]).Equal(float64(7))
}

func TestPutAndDelete(t *testing.T) {
	svr, captured := newEchoServer(t)
	type resp struct {
		OK bool `json:"ok"`
	}
	_, _, err := Put[resp](context.Background(), targetOf(svr), "/orders/1", map[string]any{"name": "p"})
	assert.That(t, err).Nil()
	assert.That(t, captured.Method).Equal(http.MethodPut)
	_, _, err = Delete[resp](context.Background(), targetOf(svr), "/orders/1")
	assert.That(t, err).Nil()
	assert.That(t, captured.Method).Equal(http.MethodDelete)
}

func TestWithQueryAndHeader(t *testing.T) {
	svr, captured := newEchoServer(t)
	type resp struct{ OK bool }
	h := http.Header{}
	h.Set("X-Trace", "abc")
	_, _, err := Get[resp](context.Background(), targetOf(svr), "/search",
		WithQuery("page=2&size=20"), httpclt.WithHeader(h))
	assert.That(t, err).Nil()
	assert.That(t, captured.Query).Equal("page=2&size=20")
	assert.That(t, captured.Header.Get("X-Trace")).Equal("abc")
}

func TestWithQueryRejectsMalformed(t *testing.T) {
	// An unencodable query string must surface as an error, not a silently
	// dropped query (url.ParseQuery rejects e.g. a bare '%').
	svr, _ := newEchoServer(t)
	type resp struct{}
	_, _, err := Get[resp](context.Background(), targetOf(svr), "/x", WithQuery("%zz"))
	assert.That(t, err).NotNil()
}

func TestCallExplicitMethodAndSchema(t *testing.T) {
	svr, captured := newEchoServer(t)
	type resp struct{ OK bool }
	_, _, err := Call[resp](context.Background(), http.MethodPatch, targetOf(svr), "/orders/1", nil, WithScheme("http"))
	assert.That(t, err).Nil()
	assert.That(t, captured.Method).Equal(http.MethodPatch)
	// WithScheme("") keeps the http default rather than clearing it.
	meta := httpclt.Metadata{}
	WithScheme("")(&meta)
	assert.That(t, meta.Schema).Equal("")
}
