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

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"golang.org/x/oauth2"

	StarterOAuth2Client "go-spring.org/starter-oauth2-client"
)

// issuedToken is the access token the fake authorization server hands out and
// the resource server expects to see on protected requests.
const issuedToken = "demo-access-token"

// expectedAudience is the extra token-endpoint parameter configured via
// endpoint-params; the fake token server asserts it arrives.
const expectedAudience = "https://api.example.com"

// Service consumes the OAuth2-backed HTTP client. The client is registered by
// the starter under the group key "downstream" (see conf/app.properties), so it
// is injected by that name. The starter also registers an oauth2.TokenSource
// under the same name, and an *oauth2.Config for the "login" authcode entry.
type Service struct {
	Client   *http.Client                     `autowire:"downstream"`
	TokenSrc *StarterOAuth2Client.TokenSource `autowire:"downstream"`
	OAuth    *oauth2.Config                   `autowire:"login"`
}

// startAuthServer runs a minimal client-credentials token endpoint on :9401.
// It ignores the (already validated) client credentials and returns a fixed
// bearer token, which is enough to exercise the starter end to end.
func startAuthServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		// The client-credentials client sends endpoint-params (here "audience")
		// in the request body; assert it flows through end to end.
		_ = r.ParseForm()
		if aud := r.Form.Get("audience"); aud != "" && aud != expectedAudience {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "unexpected audience: %q", aud)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":%q,"token_type":"Bearer","expires_in":3600}`, issuedToken)
	})
	_ = http.ListenAndServe("127.0.0.1:9401", mux)
}

// startResourceServer runs a protected downstream API on :9402 that requires
// the bearer token minted above.
func startResourceServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+issuedToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("missing or invalid bearer token"))
			return
		}
		_, _ = w.Write([]byte("hello from protected resource"))
	})
	// /api/flaky fails with 503 on its first two calls, then succeeds. It proves
	// the resilience transport retries transient 5xx responses transparently.
	mux.HandleFunc("/api/flaky", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&flakyHits, 1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered after retries"))
	})
	_ = http.ListenAndServe("127.0.0.1:9402", mux)
}

// flakyHits counts how many times /api/flaky has been hit, so the resilient
// client's retries are observable.
var flakyHits int32

func main() {
	go startAuthServer()
	go startResourceServer()

	// Register the service as a root object so the container instantiates it
	// even though nothing else depends on it.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	gs.Run()
}

func runTest(s *Service) {
	ctx := context.Background()

	// The injected client fetches a token from the auth server on the first
	// call, then attaches it as a bearer token to the downstream request.
	resp, err := s.Client.Get("http://127.0.0.1:9402/api/hello")
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "request failed: %v", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf(ctx, log.TagAppDef, "unexpected status %d: %s", resp.StatusCode, string(body))
		os.Exit(1)
	}
	if string(body) != "hello from protected resource" {
		log.Errorf(ctx, log.TagAppDef, "unexpected body: %q", string(body))
		os.Exit(1)
	}

	// Feature 2: the TokenSource exposes the raw bearer token directly.
	tok, err := s.TokenSrc.Token()
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "token source failed: %v", err)
		os.Exit(1)
	}
	if tok.AccessToken != issuedToken {
		log.Errorf(ctx, log.TagAppDef, "unexpected token: %q", tok.AccessToken)
		os.Exit(1)
	}

	// Feature 2b: after a fetch, the cached token is observable without forcing
	// another round-trip to the auth server.
	if !s.TokenSrc.Valid() {
		log.Errorf(ctx, log.TagAppDef, "expected cached token to be valid")
		os.Exit(1)
	}
	if peek := s.TokenSrc.Peek(); peek == nil || peek.AccessToken != issuedToken {
		log.Errorf(ctx, log.TagAppDef, "unexpected peeked token: %+v", peek)
		os.Exit(1)
	}

	// Feature 3: the authorization_code config builds a redirect URL.
	authURL := s.OAuth.AuthCodeURL("xyz-state")
	if !strings.Contains(authURL, "client_id=web-client") ||
		!strings.Contains(authURL, "state=xyz-state") ||
		!strings.HasPrefix(authURL, "http://127.0.0.1:9401/oauth/authorize") {
		log.Errorf(ctx, log.TagAppDef, "unexpected auth code URL: %s", authURL)
		os.Exit(1)
	}

	// Feature 4: resilience. The same injected client calls a flaky endpoint
	// that returns 503 twice before succeeding; the resilience transport retries
	// transparently, so the caller sees a single successful response.
	flakyResp, err := s.Client.Get("http://127.0.0.1:9402/api/flaky")
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "flaky request failed: %v", err)
		os.Exit(1)
	}
	flakyBody, _ := io.ReadAll(flakyResp.Body)
	_ = flakyResp.Body.Close()
	if flakyResp.StatusCode != http.StatusOK || string(flakyBody) != "recovered after retries" {
		log.Errorf(ctx, log.TagAppDef, "resilience retry did not recover: status=%d body=%q", flakyResp.StatusCode, string(flakyBody))
		os.Exit(1)
	}
	if got := atomic.LoadInt32(&flakyHits); got != 3 {
		log.Errorf(ctx, log.TagAppDef, "expected 3 attempts (2 failures + 1 success), got %d", got)
		os.Exit(1)
	}

	fmt.Println("Response from protected resource:", string(body))
	fmt.Println("Token from TokenSource:", tok.AccessToken)
	fmt.Println("Token expiry (observed):", s.TokenSrc.Expiry())
	fmt.Println("Authorization Code URL:", authURL)
	fmt.Printf("Resilience: recovered after %d attempts: %s\n", atomic.LoadInt32(&flakyHits), string(flakyBody))
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
