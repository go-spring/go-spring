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

package httpauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// guarded builds a guard-wrapped handler that records a hit.
func guarded(g Guard) (http.Handler, *bool) {
	hit := new(bool)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hit = true
		w.WriteHeader(http.StatusOK)
	})
	return g.Wrap(h), hit
}

func TestEnabled(t *testing.T) {
	assert.That(t, Guard{}.Enabled()).False()
	assert.That(t, Guard{Username: "u"}.Enabled()).False() // password missing
	assert.That(t, Guard{Password: "p"}.Enabled()).False() // username missing
	assert.That(t, Guard{Token: "t"}.Enabled()).True()
	assert.That(t, Guard{Username: "u", Password: "p"}.Enabled()).True()
}

func TestDisabledPassthrough(t *testing.T) {
	h, hit := guarded(Guard{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.That(t, rec.Code).Equal(http.StatusOK)
	assert.That(t, *hit).True()
}

func TestBearerToken(t *testing.T) {
	g := Guard{Token: "s3cret"}

	// Correct bearer header passes.
	h, hit := guarded(g)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	h.ServeHTTP(rec, req)
	assert.That(t, rec.Code).Equal(http.StatusOK)
	assert.That(t, *hit).True()

	// Wrong token, bare token (no Bearer prefix), and query-parameter
	// fallback are all rejected: only the header form is accepted.
	for _, auth := range []string{"", "Bearer wrong", "s3cret", "Basic s3cret"} {
		h, hit := guarded(g)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/?token=s3cret", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		h.ServeHTTP(rec, req)
		assert.That(t, rec.Code).Equal(http.StatusUnauthorized)
		assert.That(t, *hit).False()
	}

	// Bearer failures do not carry a WWW-Authenticate challenge.
	h, _ = guarded(g)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.That(t, rec.Header().Get("WWW-Authenticate")).Equal("")
}

func TestBasicAuth(t *testing.T) {
	g := Guard{Username: "admin", Password: "pw"}

	// Correct credentials pass.
	h, hit := guarded(g)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "pw")
	h.ServeHTTP(rec, req)
	assert.That(t, rec.Code).Equal(http.StatusOK)
	assert.That(t, *hit).True()

	// Missing or wrong credentials are rejected with a Basic challenge.
	for _, creds := range [][2]string{{"", ""}, {"admin", "bad"}, {"bad", "pw"}} {
		h, hit := guarded(g)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if creds[0] != "" {
			req.SetBasicAuth(creds[0], creds[1])
		}
		h.ServeHTTP(rec, req)
		assert.That(t, rec.Code).Equal(http.StatusUnauthorized)
		assert.That(t, rec.Header().Get("WWW-Authenticate")).Equal(`Basic realm="restricted"`)
		assert.That(t, *hit).False()
	}
}

func TestTokenTakesPrecedenceOverBasic(t *testing.T) {
	g := Guard{Token: "s3cret", Username: "admin", Password: "pw"}

	// Basic credentials do not satisfy the token guard.
	h, hit := guarded(g)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "pw")
	h.ServeHTTP(rec, req)
	assert.That(t, rec.Code).Equal(http.StatusUnauthorized)
	assert.That(t, *hit).False()

	// The bearer header does.
	h, hit = guarded(g)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	h.ServeHTTP(rec, req)
	assert.That(t, rec.Code).Equal(http.StatusOK)
	assert.That(t, *hit).True()
}

func TestConstantTimeCompares(t *testing.T) {
	// Different lengths and different same-length values both reject; the
	// guard never short-circuits on length mismatch.
	assert.That(t, constantTimeEqual("abc", "abcd")).False()
	assert.That(t, constantTimeEqual("abcd", "abc")).False()
	assert.That(t, constantTimeEqual("abc", "abd")).False()
	assert.That(t, constantTimeEqual("abc", "abc")).True()
	assert.That(t, constantTimeEqual("", "")).True()
}

func TestIsLoopback(t *testing.T) {
	// Plain loopback IPs and "localhost" are loopback.
	assert.That(t, IsLoopback("127.0.0.1:9981")).True()
	assert.That(t, IsLoopback("[::1]:9981")).True()
	assert.That(t, IsLoopback("localhost:9981")).True()
	// Wildcard and public addresses are not: they accept off-host traffic.
	assert.That(t, IsLoopback(":9981")).False()
	assert.That(t, IsLoopback("0.0.0.0:9981")).False()
	assert.That(t, IsLoopback("10.0.0.1:9981")).False()
	// A bare host without a port still classifies.
	assert.That(t, IsLoopback("127.0.0.1")).True()
	assert.That(t, IsLoopback("example.com")).False()
}
