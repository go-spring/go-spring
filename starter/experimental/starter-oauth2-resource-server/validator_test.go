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

package StarterOauth2ResourceServer

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-spring.org/cloud/security"
)

var ctx = context.Background()

// mintHS signs claims with the given HMAC secret.
func mintHS(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// mintAsym signs claims with the given RSA/ECDSA private key and kid header.
func mintAsym(t *testing.T, key any, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	if _, ok := key.(*ecdsa.PrivateKey); ok {
		tok = jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	}
	if kid != "" {
		tok.Header["kid"] = kid
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHMACSecret(t *testing.T) {
	v, err := newValidator(Config{Secret: "s3cret", ScopeClaim: "scope", RolesClaim: "roles"})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	token := mintHS(t, "s3cret", jwt.MapClaims{
		"sub": "alice", "iss": "https://auth.example.com",
		"exp": now.Add(time.Hour).Unix(), "scope": "orders:read orders:write", "roles": []any{"admin"},
	})
	auth, err := v.Validate(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Principal.Subject != "alice" || !auth.Authenticated {
		t.Fatalf("bad auth: %+v", auth)
	}
	if !auth.HasAuthority("orders:read") || !auth.HasAuthority("admin") {
		t.Fatalf("bad authorities: %v", auth.Authorities)
	}

	// A token signed with a different secret is rejected.
	if _, err := v.Validate(ctx, mintHS(t, "other", jwt.MapClaims{"sub": "x", "exp": now.Add(time.Hour).Unix()})); err == nil {
		t.Fatal("wrong-secret token accepted")
	}
}

func TestClaimValidation(t *testing.T) {
	v, err := newValidator(Config{
		Secret: "s3cret", Issuer: "https://auth.example.com", Audiences: []string{"api"},
		ScopeClaim: "scope", RolesClaim: "roles",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	base := func() jwt.MapClaims {
		return jwt.MapClaims{
			"sub": "alice", "iss": "https://auth.example.com", "aud": "api",
			"exp": now.Add(time.Hour).Unix(),
		}
	}

	cases := []struct {
		name   string
		mutate func(c jwt.MapClaims)
	}{
		{"expired", func(c jwt.MapClaims) { c["exp"] = now.Add(-time.Hour).Unix() }},
		{"not-yet-valid", func(c jwt.MapClaims) { c["nbf"] = now.Add(time.Hour).Unix() }},
		{"issuer-mismatch", func(c jwt.MapClaims) { c["iss"] = "https://evil.example.com" }},
		{"audience-mismatch", func(c jwt.MapClaims) { c["aud"] = "other-api" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			tc.mutate(c)
			if _, err := v.Validate(ctx, mintHS(t, "s3cret", c)); err == nil {
				t.Fatalf("%s token accepted", tc.name)
			}
		})
	}

	// exp within the leeway tolerance is accepted.
	vl, err := newValidator(Config{Secret: "s3cret", Leeway: 5 * time.Minute, ScopeClaim: "scope", RolesClaim: "roles"})
	if err != nil {
		t.Fatal(err)
	}
	c := base()
	c["exp"] = now.Add(-time.Minute).Unix()
	if _, err := vl.Validate(ctx, mintHS(t, "s3cret", c)); err != nil {
		t.Fatalf("expired-within-leeway rejected: %v", err)
	}
}

func TestStaticPublicKeyPEM(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey)})
	v, err := newValidator(Config{PublicKey: string(pemBytes), ScopeClaim: "scope", RolesClaim: "roles"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()

	token := mintAsym(t, rsaKey, "", jwt.MapClaims{"sub": "bob", "exp": now.Add(time.Hour).Unix()})
	if _, err := v.Validate(ctx, token); err != nil {
		t.Fatal(err)
	}

	// ES256 with an ECDSA PEM also verifies.
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&ecKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	ecPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	ve, err := newValidator(Config{PublicKey: string(ecPEM), Algorithm: "ES256", ScopeClaim: "scope", RolesClaim: "roles"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ve.Validate(ctx, mintAsym(t, ecKey, "", jwt.MapClaims{"sub": "carol", "exp": now.Add(time.Hour).Unix()})); err != nil {
		t.Fatal(err)
	}

	// Algorithm pinning rejects an otherwise-valid token signed differently.
	if _, err := v.Validate(ctx, mintHS(t, string(pemBytes), jwt.MapClaims{"sub": "mallory", "exp": now.Add(time.Hour).Unix()})); err == nil {
		t.Fatal("algorithm-confusion HMAC token accepted for PEM source")
	}
}

// fakeIdP serves /.well-known/openid-configuration and /jwks.json for the given
// RSA key, standing in for a real authorization server.
func fakeIdP(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":   srv.URL,
			"jwks_uri": srv.URL + "/jwks.json",
		})
	})
	mux.HandleFunc("/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []any{map[string]string{
				"kty": "RSA", "kid": "key-1", "use": "sig", "alg": "RS256",
				"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
			}},
		})
	})
	return srv
}

func TestIssuerDiscovery(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := fakeIdP(t, key)

	v, err := newValidator(Config{
		IssuerURI: idp.URL, Audiences: []string{"api"},
		ScopeClaim: "scope", RolesClaim: "roles",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()

	// A token the IdP signed validates end-to-end via discovery + JWKS.
	token := mintAsym(t, key, "key-1", jwt.MapClaims{
		"sub": "dave", "iss": idp.URL, "aud": "api", "exp": now.Add(time.Hour).Unix(),
	})
	auth, err := v.Validate(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Principal.Subject != "dave" {
		t.Fatalf("subject = %q", auth.Principal.Subject)
	}

	// The issuer-uri doubles as the expected "iss" claim.
	if _, err := v.Validate(ctx, mintAsym(t, key, "key-1", jwt.MapClaims{
		"sub": "eve", "iss": "https://evil.example.com", "exp": now.Add(time.Hour).Unix(),
	})); err == nil {
		t.Fatal("token with foreign issuer accepted")
	}

	// An unknown kid is rejected (after one refresh attempt).
	if _, err := v.Validate(ctx, mintAsym(t, key, "key-9", jwt.MapClaims{
		"sub": "frank", "iss": idp.URL, "exp": now.Add(time.Hour).Unix(),
	})); err == nil {
		t.Fatal("token with unknown kid accepted")
	}
}

func TestDiscoveryFailure(t *testing.T) {
	// A non-200 discovery endpoint fails construction (fail fast).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	if _, err := newValidator(Config{IssuerURI: srv.URL, ScopeClaim: "scope", RolesClaim: "roles"}); err == nil {
		t.Fatal("broken discovery endpoint did not fail construction")
	}
}

func TestKeySourceErrors(t *testing.T) {
	if _, err := newValidator(Config{ScopeClaim: "scope", RolesClaim: "roles"}); err == nil {
		t.Fatal("no key source accepted")
	}
	if _, err := newValidator(Config{IssuerURI: "https://x", Secret: "s", ScopeClaim: "scope", RolesClaim: "roles"}); err == nil {
		t.Fatal("multiple key sources accepted")
	}
}

// The Validator satisfies the framework-neutral seam.
var _ security.TokenValidator = (*Validator)(nil)
