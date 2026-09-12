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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-spring.org/stdlib/httpclt"
)

// recordingRT answers every request with its own tag so a test can tell which
// route handled it.
type recordingRT struct{ tag string }

func (r recordingRT) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(r.tag)),
		Header:     make(http.Header),
	}, nil
}

// TestRouteLooksUpByTarget pins the routing key: a call is dispatched by the
// target it declares, not by anything on the request URL. The URL here points at
// a host no entry registered — only the declared target has a route.
func TestRouteLooksUpByTarget(t *testing.T) {
	dt := &dispatchTransport{routes: map[string]http.RoundTripper{
		"greet-svc": recordingRT{tag: "declared-target"},
	}}

	rt, err := dt.transport("greet-svc")
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	resp, err := rt.RoundTrip(mustRequest(t, "http://10.0.0.9:9999/greet"))
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "declared-target" {
		t.Fatalf("expected the declared-target route, got %q", body)
	}
}

// TestRouteUnknownTarget covers the error path: the reported target is the one
// that was looked up.
func TestRouteUnknownTarget(t *testing.T) {
	dt := &dispatchTransport{routes: map[string]http.RoundTripper{}}

	_, err := dt.transport("nope-svc")
	if err == nil {
		t.Fatal("expected an error for an unconfigured target")
	}
	if !strings.Contains(err.Error(), "nope-svc") {
		t.Fatalf("error should name the target, got: %v", err)
	}
}

// TestHookRoutesByMetadataTarget proves the hook dispatches on Metadata.Target
// end to end, and that a redirect keeps the call on the route it started on:
// net/http rebuilds the redirect hop from the URL alone, so only a route that
// was chosen before the client ran can cover it.
func TestHookRoutesByMetadataTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "final")
	}))
	defer srv.Close()

	// The route is registered under a service name while the request URL carries
	// the real address — the two are deliberately different, so a lookup by URL
	// would miss.
	dt := &dispatchTransport{routes: map[string]http.RoundTripper{
		"greet-svc": http.DefaultTransport,
	}}

	prev := httpclt.DoRequest
	if _, err := newDoRequestHook(dt); err != nil {
		t.Fatal(err)
	}
	defer func() { httpclt.DoRequest = prev }()

	// Read inside the callback: DoRequest owns the body and closes it on return.
	var got string
	req := mustRequest(t, srv.URL+"/redirect")
	if _, err := httpclt.DoRequest(req, httpclt.Metadata{Target: "greet-svc"}, func(r io.Reader) error {
		b, err := io.ReadAll(r)
		got = string(b)
		return err
	}); err != nil {
		t.Fatalf("request through the hook failed: %v", err)
	}
	if got != "final" {
		t.Fatalf("expected the redirect to be followed, got %q", got)
	}
}

// TestHookRejectsUnknownTarget proves an unconfigured target fails at the call
// site with the target named, rather than dialing something arbitrary.
func TestHookRejectsUnknownTarget(t *testing.T) {
	dt := &dispatchTransport{routes: map[string]http.RoundTripper{}}

	prev := httpclt.DoRequest
	if _, err := newDoRequestHook(dt); err != nil {
		t.Fatal(err)
	}
	defer func() { httpclt.DoRequest = prev }()

	req := mustRequest(t, "http://10.0.0.9:9999/ping")
	_, err := httpclt.DoRequest(req, httpclt.Metadata{Target: "unregistered"}, func(io.Reader) error { return nil })
	if err == nil {
		t.Fatal("expected an error for an unconfigured target")
	}
	if !strings.Contains(err.Error(), "unregistered") {
		t.Fatalf("error should name the target, got: %v", err)
	}
}

// TestHookWithoutRouteTable proves a route table that failed to assemble is
// reported as a construct error rather than panicking inside the hook.
func TestHookWithoutRouteTable(t *testing.T) {
	if _, err := newDoRequestHook(nil); err == nil {
		t.Fatal("expected an error when the route table is missing")
	}
}

func mustRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
