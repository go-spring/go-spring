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

package StarterPProf

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// firedSignal is a gs.ReadySignal that reports ready immediately, so the
// server's Run starts serving as soon as the listener is up.
type firedSignal struct{}

func (firedSignal) TriggerAndWait() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// startPProf starts a real SimplePProfServer on a loopback port and returns
// its base URL. The server is stopped when the test ends.
func startPProf(t *testing.T, c Config) string {
	t.Helper()

	// Grab a free port up front because the server's address is fixed at
	// construction time and Run does not expose the bound listener. Passing
	// ":0" would make the server pick a different random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.That(t, err).Nil()
	addr := ln.Addr().String()
	assert.That(t, ln.Close()).Nil()
	c.Address = addr

	svr := NewSimplePProfServer(&gs.ContextProvider{Context: context.Background()}, c)
	go func() { _ = svr.Run(context.Background(), firedSignal{}) }()
	t.Cleanup(func() { _ = svr.Stop() })

	base := "http://" + addr
	for i := 0; i < 100; i++ {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return base
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pprof server on %s never became reachable", addr)
	return ""
}

// doReq sends req and returns the response status and body.
func doReq(t *testing.T, req *http.Request) (int, string) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	assert.That(t, err).Nil()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	assert.That(t, err).Nil()
	return resp.StatusCode, string(body)
}

// get fetches path with the given authorization header, returning status and
// body.
func get(t *testing.T, url, authorization string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	assert.That(t, err).Nil()
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return doReq(t, req)
}

func TestPProfEndpointsNoAuth(t *testing.T) {
	// Loopback binding without any auth: the guard is a no-op and every
	// pprof endpoint is reachable.
	base := startPProf(t, Config{Address: "127.0.0.1:0"})

	code, body := get(t, base+"/debug/pprof/", "")
	assert.That(t, code).Equal(http.StatusOK)
	assert.That(t, strings.Contains(body, "heap")).True()

	code, body = get(t, base+"/debug/pprof/cmdline", "")
	assert.That(t, code).Equal(http.StatusOK)
	assert.That(t, strings.Contains(body, os.Args[0])).True()

	code, _ = get(t, base+"/debug/pprof/symbol", "")
	assert.That(t, code).Equal(http.StatusOK)

	code, body = get(t, base+"/debug/pprof/goroutine?debug=1", "")
	assert.That(t, code).Equal(http.StatusOK)
	assert.That(t, strings.Contains(body, "goroutine")).True()

	// Routes are registered as GET only, so other methods are rejected by
	// the mux instead of silently falling through.
	postReq, err := http.NewRequest(http.MethodPost, base+"/debug/pprof/", nil)
	assert.That(t, err).Nil()
	code, _ = doReq(t, postReq)
	assert.That(t, code).Equal(http.StatusMethodNotAllowed)
}

func TestPProfTokenAuth(t *testing.T) {
	base := startPProf(t, Config{Address: "127.0.0.1:0", Token: "s3cret"})

	// No credentials and a wrong token are rejected with 401.
	code, _ := get(t, base+"/debug/pprof/", "")
	assert.That(t, code).Equal(http.StatusUnauthorized)
	code, _ = get(t, base+"/debug/pprof/", "Bearer wrong")
	assert.That(t, code).Equal(http.StatusUnauthorized)

	// A valid bearer header authenticates.
	code, _ = get(t, base+"/debug/pprof/", "Bearer s3cret")
	assert.That(t, code).Equal(http.StatusOK)

	// The token query parameter is NOT accepted: only the header form is.
	code, _ = get(t, base+"/debug/pprof/?token=s3cret", "")
	assert.That(t, code).Equal(http.StatusUnauthorized)

	// Token takes precedence: Basic credentials do not satisfy it.
	req, err := http.NewRequest(http.MethodGet, base+"/debug/pprof/", nil)
	assert.That(t, err).Nil()
	req.SetBasicAuth("admin", "pw")
	code, _ = doReq(t, req)
	assert.That(t, code).Equal(http.StatusUnauthorized)
}

func TestPProfBasicAuth(t *testing.T) {
	base := startPProf(t, Config{Address: "127.0.0.1:0", Username: "admin", Password: "pw"})

	// No credentials: 401 plus a WWW-Authenticate challenge.
	code, _ := get(t, base+"/debug/pprof/", "")
	assert.That(t, code).Equal(http.StatusUnauthorized)

	// A wrong password is rejected.
	req, err := http.NewRequest(http.MethodGet, base+"/debug/pprof/", nil)
	assert.That(t, err).Nil()
	req.SetBasicAuth("admin", "bad")
	code, _ = doReq(t, req)
	assert.That(t, code).Equal(http.StatusUnauthorized)

	// Correct credentials authenticate.
	req, err = http.NewRequest(http.MethodGet, base+"/debug/pprof/", nil)
	assert.That(t, err).Nil()
	req.SetBasicAuth("admin", "pw")
	code, _ = doReq(t, req)
	assert.That(t, code).Equal(http.StatusOK)
}

func TestPProfWarnPathsDoNotPanic(t *testing.T) {
	// The wildcard default without auth logs a warning; construction must
	// still succeed. The server is never started, so the port is safe.
	svr := NewSimplePProfServer(&gs.ContextProvider{Context: context.Background()}, Config{})
	assert.That(t, svr).NotNil()

	// Loopback binding suppresses the warning; construction succeeds too.
	svr = NewSimplePProfServer(&gs.ContextProvider{Context: context.Background()}, Config{Address: "127.0.0.1:0"})
	assert.That(t, svr).NotNil()
}
