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

package StarterHTTPServer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-spring.org/cloud/runtime"
	"go-spring.org/cloud/runtime/dataplane/guard"
)

// serveServerPolicy drives one request through the inbound filter and reports the
// response plus how often the handler ran. The inbound authority is armed with
// the empty document, so governance never rejects — the tests are about the
// filter's pass-through behavior over the process-wide no-op providers.
func serveServerPolicy(t *testing.T, handler http.HandlerFunc) (*httptest.ResponseRecorder, int) {
	t.Helper()
	handlerRuns := 0
	mgr := guard.NewManager()
	in := runtime.InboundOver("http-server", "http-server::9090", mgr)
	mw := serverPolicyWith(in)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerRuns++
		handler(w, r)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	return rec, handlerRuns
}

func TestServerPolicy_PassThroughRunsHandlerOnce(t *testing.T) {
	rec, handlerRuns := serveServerPolicy(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if handlerRuns != 1 {
		t.Fatalf("handler runs: got %d, want 1", handlerRuns)
	}
}

// A handler's own 5xx response must survive the filter untouched.
func TestServerPolicy_Handler5xxSurvives(t *testing.T) {
	rec, _ := serveServerPolicy(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("the handler's own response must survive: got %d, want 500", rec.Code)
	}
}

// The wrapper must stay transparent for the interfaces handlers rely on:
// http.ResponseController reaches the original writer through Unwrap, and
// http.Flusher is forwarded.
func TestServerPolicy_StatusWriterStaysTransparent(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}

	if got := sw.Unwrap(); got != http.ResponseWriter(rec) {
		t.Fatalf("Unwrap must return the original writer, got %T", got)
	}
	if _, ok := any(sw).(http.Flusher); !ok {
		t.Fatal("statusWriter must implement http.Flusher")
	}
	sw.WriteHeader(http.StatusTeapot)
	if sw.status != http.StatusTeapot || !sw.written {
		t.Fatalf("WriteHeader should be recorded: status=%d written=%v", sw.status, sw.written)
	}
	// A second WriteHeader must not overwrite the recorded status, matching
	// net/http's own "first header wins" rule.
	sw.WriteHeader(http.StatusOK)
	if sw.status != http.StatusTeapot {
		t.Fatalf("first WriteHeader wins: got %d, want 418", sw.status)
	}
}
