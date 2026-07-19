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

package StarterSecurityJWT

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-spring.org/stdlib/security"
	"go-spring.org/stdlib/testing/assert"
)

// signHS mints an HS256 token signed with secret.
func signHS(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	assert.Error(t, err).Nil()
	return s
}

// signRS mints an RS256 token and returns it alongside the PEM public key.
func signRS(t *testing.T, claims jwt.MapClaims) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.Error(t, err).Nil()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := tok.SignedString(key)
	assert.Error(t, err).Nil()
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	assert.Error(t, err).Nil()
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return s, string(pubPEM)
}

func TestConfig_Source(t *testing.T) {
	_, err := (Config{}).source()
	assert.Error(t, err).Matches("no verification key source")

	_, err = Config{Secret: "s", JWKSURL: "http://x"}.source()
	assert.Error(t, err).Matches("multiple verification key sources")

	src, err := Config{Secret: "s"}.source()
	assert.Error(t, err).Nil()
	assert.That(t, src).Equal(sourceHMAC)
}

func TestValidMethods(t *testing.T) {
	// HMAC source without pin allows only HS*.
	m, err := validMethods(Config{}, sourceHMAC)
	assert.Error(t, err).Nil()
	assert.Slice(t, m).Equal([]string{"HS256", "HS384", "HS512"})

	// Pinning an HMAC alg on an asymmetric source is rejected — this is the
	// algorithm-confusion guard.
	_, err = validMethods(Config{Algorithm: "HS256"}, sourcePEM)
	assert.Error(t, err).Matches("not compatible with the configured key source")

	// Pinning a compatible alg narrows to exactly that alg.
	m, err = validMethods(Config{Algorithm: "rs256"}, sourcePEM)
	assert.Error(t, err).Nil()
	assert.Slice(t, m).Equal([]string{"RS256"})
}

func TestAuthenticator_ValidateHS(t *testing.T) {
	a, err := newAuthenticator(Config{
		Secret:     "topsecret",
		Issuer:     "iss-a",
		ScopeClaim: "scope",
		RolesClaim: "roles",
	})
	assert.Error(t, err).Nil()

	token := signHS(t, "topsecret", jwt.MapClaims{
		"sub":   "user-1",
		"iss":   "iss-a",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"scope": "read write",
		"roles": []string{"admin"},
	})
	auth, err := a.Validate(t.Context(), token)
	assert.Error(t, err).Nil()
	assert.String(t, auth.Principal.Subject).Equal("user-1")
	assert.That(t, auth.Authenticated).True()
	assert.That(t, auth.HasAuthority("read")).True()
	assert.That(t, auth.HasAuthority("admin")).True()
	assert.That(t, auth.HasAuthority("delete")).False()
}

func TestAuthenticator_RejectsWrongIssuer(t *testing.T) {
	a, err := newAuthenticator(Config{Secret: "s", Issuer: "iss-a"})
	assert.Error(t, err).Nil()
	token := signHS(t, "s", jwt.MapClaims{"iss": "iss-b", "exp": time.Now().Add(time.Hour).Unix()})
	_, err = a.Validate(t.Context(), token)
	assert.Error(t, err).Matches("iss")
}

func TestAuthenticator_RejectsExpired(t *testing.T) {
	a, err := newAuthenticator(Config{Secret: "s"})
	assert.Error(t, err).Nil()
	token := signHS(t, "s", jwt.MapClaims{"exp": time.Now().Add(-time.Hour).Unix()})
	_, err = a.Validate(t.Context(), token)
	assert.Error(t, err).Matches("expired")
}

func TestAuthenticator_Audience(t *testing.T) {
	a, err := newAuthenticator(Config{Secret: "s", Audience: []string{"svc-a"}})
	assert.Error(t, err).Nil()

	ok := signHS(t, "s", jwt.MapClaims{"aud": "svc-a", "exp": time.Now().Add(time.Hour).Unix()})
	_, err = a.Validate(t.Context(), ok)
	assert.Error(t, err).Nil()

	bad := signHS(t, "s", jwt.MapClaims{"aud": "svc-b", "exp": time.Now().Add(time.Hour).Unix()})
	_, err = a.Validate(t.Context(), bad)
	assert.Error(t, err).Matches("audience")
}

func TestAuthenticator_ValidatePEM(t *testing.T) {
	token, pubPEM := signRS(t, jwt.MapClaims{"sub": "u", "exp": time.Now().Add(time.Hour).Unix()})
	a, err := newAuthenticator(Config{PublicKey: pubPEM})
	assert.Error(t, err).Nil()
	auth, err := a.Validate(t.Context(), token)
	assert.Error(t, err).Nil()
	assert.String(t, auth.Principal.Subject).Equal("u")
}

// TestAuthenticator_AlgConfusion proves an attacker cannot sign with the RSA
// public key bytes as an HMAC secret against a PEM-configured authenticator.
func TestAuthenticator_AlgConfusion(t *testing.T) {
	_, pubPEM := signRS(t, jwt.MapClaims{"sub": "u"})
	a, err := newAuthenticator(Config{PublicKey: pubPEM})
	assert.Error(t, err).Nil()
	forged := signHS(t, pubPEM, jwt.MapClaims{"sub": "attacker", "exp": time.Now().Add(time.Hour).Unix()})
	_, err = a.Validate(t.Context(), forged)
	assert.Error(t, err).NotNil()
}

func TestWrap_MissingTokenRequired(t *testing.T) {
	a, err := newAuthenticator(Config{Secret: "s", Required: true})
	assert.Error(t, err).Nil()
	h := a.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.That(t, rec.Code).Equal(http.StatusUnauthorized)
}

func TestWrap_MissingTokenOptional(t *testing.T) {
	a, err := newAuthenticator(Config{Secret: "s", Required: false})
	assert.Error(t, err).Nil()
	var reached bool
	h := a.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		_, ok := security.FromContext(r.Context())
		assert.That(t, ok).False() // no identity attached
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.That(t, reached).True()
	assert.That(t, rec.Code).Equal(http.StatusOK)
}

func TestWrap_ValidTokenAttachesAuth(t *testing.T) {
	a, err := newAuthenticator(Config{Secret: "s"})
	assert.Error(t, err).Nil()
	token := signHS(t, "s", jwt.MapClaims{"sub": "u", "exp": time.Now().Add(time.Hour).Unix()})

	var subject string
	h := a.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth, ok := security.FromContext(r.Context()); ok {
			subject = auth.Principal.Subject
		}
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.That(t, rec.Code).Equal(http.StatusOK)
	assert.String(t, subject).Equal("u")
}

func TestWrap_InvalidToken401(t *testing.T) {
	a, err := newAuthenticator(Config{Secret: "s"})
	assert.Error(t, err).Nil()
	h := a.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.That(t, rec.Code).Equal(http.StatusUnauthorized)
}

func TestClaimStrings(t *testing.T) {
	assert.Slice(t, claimStrings("a b c")).Equal([]string{"a", "b", "c"})
	assert.Slice(t, claimStrings([]any{"a", "b"})).Equal([]string{"a", "b"})
	assert.Slice(t, claimStrings([]string{"x"})).Equal([]string{"x"})
	assert.That(t, claimStrings(nil)).Nil()
}

// verify Authenticator satisfies the driver seam.
var _ security.TokenValidator = (*Authenticator)(nil)
