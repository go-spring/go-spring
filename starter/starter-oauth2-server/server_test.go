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

package StarterOAuth2Server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-shared-secret"

// newTestServer builds an HMAC-signing authorization server with a public
// (PKCE) client "spa" and a confidential client "svc", plus a stub UserAuthFunc.
func newTestServer(t *testing.T) *AuthServer {
	t.Helper()
	cfg := Config{
		Issuer:          "https://issuer.test",
		Secret:          testSecret,
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		CodeTTL:         time.Minute,
		Clients: map[string]ClientConfig{
			"spa": {
				Public:       true,
				RedirectURIs: []string{"https://client.test/callback"},
				Scopes:       []string{"read", "write"},
			},
			"svc": {
				Secret:     "svc-secret",
				Scopes:     []string{"read"},
				GrantTypes: []string{"client_credentials"},
			},
		},
	}
	s, err := newAuthServer(cfg)
	if err != nil {
		t.Fatalf("newAuthServer: %v", err)
	}
	s.UserAuthFunc = func(*http.Request) (string, []string, bool) {
		return "alice", []string{"admin"}, true
	}
	return s
}

// parseToken verifies an HS256 token against the shared secret and returns claims.
func parseToken(t *testing.T, token string) jwt.MapClaims {
	t.Helper()
	claims := jwt.MapClaims{}
	_, err := jwt.NewParser().ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("parse issued token: %v", err)
	}
	return claims
}

func TestAuthorizationCodePKCEFlow(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	verifier := GenerateVerifier()
	challenge := Challenge(verifier, "S256")

	// 1. /authorize returns a redirect carrying a code and echoing state.
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {"spa"},
		"redirect_uri":          {"https://client.test/callback"},
		"scope":                 {"read"},
		"state":                 {"xyz"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	loc := authorizeRedirect(t, srv.URL, q)
	if loc.Query().Get("state") != "xyz" {
		t.Fatalf("state not echoed: %q", loc.Query().Get("state"))
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatal("no code in redirect")
	}

	// 2. /token exchanges the code (with the verifier) for tokens.
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"https://client.test/callback"},
		"client_id":     {"spa"},
		"code_verifier": {verifier},
	}
	tok := postToken(t, srv.URL, form, http.StatusOK)
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Fatalf("expected access+refresh tokens, got %+v", tok)
	}
	claims := parseToken(t, tok.AccessToken)
	if claims["sub"] != "alice" || claims["scope"] != "read" || claims["iss"] != "https://issuer.test" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	// 3. refresh_token mints a fresh access token.
	rform := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tok.RefreshToken},
		"client_id":     {"spa"},
	}
	tok2 := postToken(t, srv.URL, rform, http.StatusOK)
	if tok2.AccessToken == "" {
		t.Fatal("refresh returned no access token")
	}

	// 4. The rotated refresh token is single-use: replaying the old one fails.
	postToken(t, srv.URL, rform, http.StatusBadRequest)
}

func TestAuthorizationCodePublicClientRequiresPKCE(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// No code_challenge for a public client is redirected back as invalid_request.
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {"spa"},
		"redirect_uri":  {"https://client.test/callback"},
		"state":         {"s"},
	}
	loc := authorizeRedirect(t, srv.URL, q)
	if loc.Query().Get("error") != "invalid_request" {
		t.Fatalf("expected invalid_request, got %q", loc.Query().Get("error"))
	}
}

func TestPKCEVerificationFailure(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	verifier := GenerateVerifier()
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {"spa"},
		"redirect_uri":          {"https://client.test/callback"},
		"code_challenge":        {Challenge(verifier, "S256")},
		"code_challenge_method": {"S256"},
	}
	code := authorizeRedirect(t, srv.URL, q).Query().Get("code")

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"https://client.test/callback"},
		"client_id":     {"spa"},
		"code_verifier": {"the-wrong-verifier"},
	}
	postToken(t, srv.URL, form, http.StatusBadRequest)
}

func TestClientCredentialsFlow(t *testing.T) {
	s := newTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"svc"},
		"client_secret": {"svc-secret"},
		"scope":         {"read"},
	}
	tok := postToken(t, srv.URL, form, http.StatusOK)
	if tok.RefreshToken != "" {
		t.Fatal("client_credentials must not issue a refresh token")
	}
	claims := parseToken(t, tok.AccessToken)
	if claims["sub"] != "svc" || claims["client_id"] != "svc" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	// A wrong secret is rejected.
	bad := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"svc"},
		"client_secret": {"nope"},
	}
	postToken(t, srv.URL, bad, http.StatusUnauthorized)

	// svc is not allowed the authorization_code grant.
	notAllowed := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"svc"},
		"client_secret": {"svc-secret"},
		"code":          {"whatever"},
	}
	postToken(t, srv.URL, notAllowed, http.StatusBadRequest)
}

func TestJWKSHMACIsEmpty(t *testing.T) {
	s := newTestServer(t)
	doc := fetchJWKS(t, httptest.NewServer(s.Handler()))
	if len(doc.Keys) != 0 {
		t.Fatalf("HMAC signer should publish no keys, got %d", len(doc.Keys))
	}
}

func TestJWKSRSAPublishesKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen rsa: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})

	s, err := newAuthServer(Config{
		PrivateKey:     string(pemBytes),
		KeyID:          "k1",
		AccessTokenTTL: time.Hour,
		Clients:        map[string]ClientConfig{},
	})
	if err != nil {
		t.Fatalf("newAuthServer(RSA): %v", err)
	}
	doc := fetchJWKS(t, httptest.NewServer(s.Handler()))
	if len(doc.Keys) != 1 {
		t.Fatalf("expected 1 published key, got %d", len(doc.Keys))
	}
	k := doc.Keys[0]
	if k.Kty != "RSA" || k.Kid != "k1" || k.Alg != "RS256" || k.N == "" || k.E == "" {
		t.Fatalf("unexpected jwk: %+v", k)
	}
}

func TestSigningKeyFailFast(t *testing.T) {
	if _, err := newAuthServer(Config{}); err == nil {
		t.Fatal("no signing key should fail")
	}
	if _, err := newAuthServer(Config{Secret: "s", PrivateKey: "p"}); err == nil {
		t.Fatal("two signing keys should fail")
	}
}

// --- helpers ---

// noRedirectClient returns an HTTP client that does not follow redirects, so a
// 302 from /authorize can be inspected.
func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func authorizeRedirect(t *testing.T, base string, q url.Values) *url.URL {
	t.Helper()
	resp, err := noRedirectClient().Get(base + "/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("GET /authorize: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("/authorize status = %d, want 302", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	return loc
}

func postToken(t *testing.T, base string, form url.Values, wantStatus int) tokenResponse {
	t.Helper()
	resp, err := http.Post(base+"/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST /token: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != wantStatus {
		t.Fatalf("/token status = %d, want %d", resp.StatusCode, wantStatus)
	}
	var tok tokenResponse
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
			t.Fatalf("decode token response: %v", err)
		}
	}
	return tok
}

func fetchJWKS(t *testing.T, srv *httptest.Server) struct {
	Keys []jwk `json:"keys"`
} {
	t.Helper()
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/jwks")
	if err != nil {
		t.Fatalf("GET /jwks: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("decode jwks: %v", err)
	}
	return doc
}
