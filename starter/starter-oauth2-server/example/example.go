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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterOAuth2Server "go-spring.org/starter-oauth2-server"
	"go-spring.org/spring/security"
)

// secret matches spring.oauth2.server.secret in conf/app.properties. The
// authorization server signs HS256 tokens with it, and the resource-server
// validator below verifies with the same key — no external identity provider.
const secret = "example-shared-secret"

const base = "http://127.0.0.1:9090"

func main() {
	// Build the application's single HTTP handler. It mounts the authorization
	// server's endpoints under /oauth2 and protects the business API with the
	// unified security filter chain (CORS + authentication + authorization).
	gs.Provide(func(as *StarterOAuth2Server.AuthServer) *gs.HttpServeMux {
		// The seam for the resource-owner login: this example simulates an
		// already-authenticated user "alice" holding the "admin" authority. A real
		// app would check its session/login here.
		as.UserAuthFunc = func(*http.Request) (string, []string, bool) {
			return "alice", []string{"admin"}, true
		}

		validator := hmacValidator{secret: []byte(secret)}
		cors := security.CORS(security.CORSConfig{
			AllowedOrigins: []string{"https://app.example.com"},
			AllowedHeaders: []string{"Authorization", "Content-Type"},
			MaxAge:         600,
		})

		mux := http.NewServeMux()

		// Authorization server: /oauth2/authorize, /oauth2/token, /oauth2/jwks.
		mux.Handle("/oauth2/", http.StripPrefix("/oauth2", as.Handler()))

		// Resource server: /api/me requires a valid token; /api/admin additionally
		// requires the "admin" authority — the ordered chain authenticates before
		// it authorizes.
		mux.Handle("/api/me", security.Chain(cors, security.Authenticate(validator, true))(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				a, _ := security.FromContext(r.Context())
				_, _ = fmt.Fprintf(w, "hello %s", a.Principal.Subject)
			})))
		mux.Handle("/api/admin", security.Chain(cors, security.Authenticate(validator, true), security.Authorize("admin"))(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("admin ok"))
			})))

		// A browser-style, cookie-based route guarded by CSRF double-submit.
		mux.Handle("/session", security.CSRF(security.CSRFConfig{})(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("session ok"))
			})))

		return &gs.HttpServeMux{Handler: mux}
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest()
	}()

	gs.Run()
}

// hmacValidator is the resource server: it verifies HS256 tokens minted by the
// authorization server and maps their scope/roles claims to authorities. It
// implements security.TokenValidator, so the stdlib Authenticate middleware can
// use it directly.
type hmacValidator struct{ secret []byte }

func (v hmacValidator) Validate(_ context.Context, token string) (*security.Authentication, error) {
	claims := jwt.MapClaims{}
	tok, err := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"})).
		ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return v.secret, nil })
	if err != nil || !tok.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	subject, _ := claims["sub"].(string)
	authorities := append(claimStrings(claims["scope"]), claimStrings(claims["roles"])...)
	return &security.Authentication{
		Principal:     security.Principal{Subject: subject, Claims: claims},
		Token:         token,
		Authenticated: true,
		Authorities:   authorities,
	}, nil
}

// claimStrings normalizes a scope/role claim (space-delimited string or array).
func claimStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return strings.Fields(t)
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func runTest() {
	// 1. Drive the authorization_code + PKCE flow as a public client would.
	verifier := StarterOAuth2Server.GenerateVerifier()
	challenge := StarterOAuth2Server.Challenge(verifier, "S256")

	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {"spa"},
		"redirect_uri":          {base + "/callback"},
		"scope":                 {"read write"},
		"state":                 {"xyz"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	code := authorizeCode(q, "xyz")

	// 2. Exchange the code (with the verifier) for tokens.
	tok := exchange(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {base + "/callback"},
		"client_id":     {"spa"},
		"code_verifier": {verifier},
	})
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		fail("authorization_code: missing tokens: %+v", tok)
	}

	// 3. The resource server accepts the issued token and echoes the subject.
	if status, body := apiGet("/api/me", tok.AccessToken); status != 200 || body != "hello alice" {
		fail("/api/me: status=%d body=%q", status, body)
	}
	// alice carries the admin authority, so /api/admin is allowed.
	if status, body := apiGet("/api/admin", tok.AccessToken); status != 200 || body != "admin ok" {
		fail("/api/admin: status=%d body=%q", status, body)
	}
	// No token is rejected.
	if status, _ := apiGet("/api/me", ""); status != http.StatusUnauthorized {
		fail("/api/me no-token: expected 401, got %d", status)
	}

	// 4. refresh_token mints a fresh access token.
	tok2 := exchange(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tok.RefreshToken},
		"client_id":     {"spa"},
	})
	if tok2.AccessToken == "" {
		fail("refresh_token: missing access token")
	}

	// 5. A wrong PKCE verifier is rejected.
	q2 := url.Values{
		"response_type":         {"code"},
		"client_id":             {"spa"},
		"redirect_uri":          {base + "/callback"},
		"code_challenge":        {StarterOAuth2Server.Challenge(StarterOAuth2Server.GenerateVerifier(), "S256")},
		"code_challenge_method": {"S256"},
	}
	badCode := authorizeCode(q2, "")
	if status := exchangeStatus(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {badCode},
		"redirect_uri":  {base + "/callback"},
		"client_id":     {"spa"},
		"code_verifier": {"the-wrong-verifier"},
	}); status != http.StatusBadRequest {
		fail("PKCE mismatch: expected 400, got %d", status)
	}

	// 6. client_credentials grant for the confidential service client.
	svcTok := exchange(url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"svc"},
		"client_secret": {"svc-secret"},
		"scope":         {"read"},
	})
	if svcTok.AccessToken == "" || svcTok.RefreshToken != "" {
		fail("client_credentials: want access token and no refresh, got %+v", svcTok)
	}

	// 7. /jwks responds (empty key set for HMAC signing).
	if status := getStatus("/oauth2/jwks"); status != 200 {
		fail("/oauth2/jwks: expected 200, got %d", status)
	}

	// 8. CORS preflight is answered directly with the allow headers.
	checkCORSPreflight()

	// 9. CSRF: a safe GET issues the cookie; an unsafe POST needs to echo it.
	checkCSRF()

	fmt.Println("Response from server: authorization_code+PKCE issued a token the resource server accepted, refresh and client_credentials worked, PKCE mismatch was rejected, and CORS/CSRF middleware enforced the filter chain")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// tokenResponse mirrors the server's token JSON.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// noRedirect is an HTTP client that does not follow redirects, so /authorize's
// 302 can be inspected.
var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}}

// authorizeCode issues the /authorize request and returns the code from the
// redirect, asserting the state is echoed when wantState is non-empty.
func authorizeCode(q url.Values, wantState string) string {
	resp, err := noRedirect.Get(base + "/oauth2/authorize?" + q.Encode())
	if err != nil {
		fail("GET /authorize: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		fail("/authorize: expected 302, got %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		fail("parse redirect: %v", err)
	}
	if wantState != "" && loc.Query().Get("state") != wantState {
		fail("state not echoed: %q", loc.Query().Get("state"))
	}
	code := loc.Query().Get("code")
	if code == "" {
		fail("no code in redirect: %s", loc)
	}
	return code
}

func exchange(form url.Values) tokenResponse {
	resp, err := http.Post(base+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		fail("POST /token: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		fail("/token: status=%d body=%s", resp.StatusCode, b)
	}
	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		fail("decode token: %v", err)
	}
	return tok
}

func exchangeStatus(form url.Values) int {
	resp, err := http.Post(base+"/oauth2/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		fail("POST /token: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func apiGet(path, token string) (int, string) {
	req, _ := http.NewRequest(http.MethodGet, base+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func getStatus(path string) int {
	resp, err := http.Get(base + path)
	if err != nil {
		fail("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func checkCORSPreflight() {
	req, _ := http.NewRequest(http.MethodOptions, base+"/api/me", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("CORS preflight: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		fail("CORS preflight: expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		fail("CORS preflight: missing allow-origin")
	}
}

func checkCSRF() {
	// A safe GET issues the csrf_token cookie.
	resp, err := http.Get(base + "/session")
	if err != nil {
		fail("GET /session: %v", err)
	}
	_ = resp.Body.Close()
	var token string
	for _, c := range resp.Cookies() {
		if c.Name == "csrf_token" {
			token = c.Value
		}
	}
	if token == "" {
		fail("CSRF: no cookie issued")
	}

	// An unsafe POST without the header is rejected.
	req, _ := http.NewRequest(http.MethodPost, base+"/session", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	r1, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("CSRF POST: %v", err)
	}
	_ = r1.Body.Close()
	if r1.StatusCode != http.StatusForbidden {
		fail("CSRF POST without header: expected 403, got %d", r1.StatusCode)
	}

	// Echoing the cookie in the header passes.
	req, _ = http.NewRequest(http.MethodPost, base+"/session", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("CSRF POST: %v", err)
	}
	_ = r2.Body.Close()
	if r2.StatusCode != http.StatusOK {
		fail("CSRF POST with header: expected 200, got %d", r2.StatusCode)
	}
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}
