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
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go-spring.org/spring/web/security"
	"go-spring.org/stdlib/errutil"
)

var (
	errNoKeySource        = errors.New("security-jwt: no verification key source configured (set one of secret, public-key/public-key-file, jwks-url)")
	errMultipleKeySources = errors.New("security-jwt: multiple verification key sources configured (set exactly one of secret, public-key/public-key-file, jwks-url)")
	errAudience           = errors.New("security-jwt: token audience not accepted")
)

// Authenticator verifies JWT bearer tokens for a resource server. It is a
// Contributor-form bean: it owns no port. The application mounts Wrap onto its
// already-running web server, so requests are authenticated before reaching
// business handlers. It also implements security.TokenValidator, so it can be
// injected and used to verify tokens programmatically (e.g. on non-HTTP
// transports).
type Authenticator struct {
	cfg     Config
	parser  *jwt.Parser
	keyfunc jwt.Keyfunc
	jwks    *jwksCache // non-nil only for the JWKS source
}

// newAuthenticator builds an Authenticator from its configuration, failing fast
// when the key source is ambiguous, the PEM key cannot be parsed, or the JWKS
// endpoint cannot be reached at startup.
func newAuthenticator(c Config) (*Authenticator, error) {
	src, err := c.source()
	if err != nil {
		return nil, err
	}

	methods, err := validMethods(c, src)
	if err != nil {
		return nil, err
	}

	a := &Authenticator{cfg: c}

	switch src {
	case sourceHMAC:
		secret := []byte(c.Secret)
		a.keyfunc = func(*jwt.Token) (any, error) { return secret, nil }
	case sourcePEM:
		key, err := parsePEMPublicKey(c)
		if err != nil {
			return nil, err
		}
		a.keyfunc = func(*jwt.Token) (any, error) { return key, nil }
	case sourceJWKS:
		cache, err := newJWKSCache(c.JWKSURL, c.JWKSRefresh, c.JWKSTimeout)
		if err != nil {
			return nil, err
		}
		a.jwks = cache
		a.keyfunc = func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			return cache.key(kid)
		}
	}

	opts := []jwt.ParserOption{
		jwt.WithValidMethods(methods),
		jwt.WithLeeway(c.Leeway),
	}
	if c.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(c.Issuer))
	}
	a.parser = jwt.NewParser(opts...)
	return a, nil
}

// validMethods returns the signing algorithms accepted for the key source. When
// Algorithm is pinned it must be compatible with the source; otherwise the full
// compatible set is allowed. HMAC algorithms are never allowed for an asymmetric
// source, which blocks the classic "sign with the public key as an HMAC secret"
// algorithm-confusion attack.
func validMethods(c Config, src keySource) ([]string, error) {
	asymmetric := []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512"}
	hmac := []string{"HS256", "HS384", "HS512"}

	allowed := asymmetric
	if src == sourceHMAC {
		allowed = hmac
	}
	if c.Algorithm == "" {
		return allowed, nil
	}
	alg := strings.ToUpper(c.Algorithm)
	if slices.Contains(allowed, alg) {
		return []string{alg}, nil
	}
	return nil, fmt.Errorf("security-jwt: algorithm %q is not compatible with the configured key source", c.Algorithm)
}

// parsePEMPublicKey loads the PEM public key from the inline value or the file,
// trying RSA then ECDSA.
func parsePEMPublicKey(c Config) (any, error) {
	pem := []byte(c.PublicKey)
	if c.PublicKeyFile != "" {
		b, err := os.ReadFile(c.PublicKeyFile)
		if err != nil {
			return nil, errutil.Explain(err, "security-jwt: read public-key-file %s", c.PublicKeyFile)
		}
		pem = b
	}
	if rsaKey, err := jwt.ParseRSAPublicKeyFromPEM(pem); err == nil {
		return rsaKey, nil
	}
	if ecKey, err := jwt.ParseECPublicKeyFromPEM(pem); err == nil {
		return ecKey, nil
	}
	return nil, errors.New("security-jwt: public key is neither a valid RSA nor ECDSA PEM")
}

// Validate verifies a raw token string and returns the security.Authentication
// it represents. It implements security.TokenValidator.
func (a *Authenticator) Validate(_ context.Context, token string) (*security.Authentication, error) {
	claims := jwt.MapClaims{}
	tok, err := a.parser.ParseWithClaims(token, claims, a.keyfunc)
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errutil.Explain(security.ErrUnauthenticated, "security-jwt: invalid token")
	}
	if len(a.cfg.Audience) > 0 && !audienceAccepted(claims, a.cfg.Audience) {
		return nil, errAudience
	}

	subject, _ := claims["sub"].(string)
	authorities := append(claimStrings(claims[a.cfg.ScopeClaim]), claimStrings(claims[a.cfg.RolesClaim])...)

	return &security.Authentication{
		Principal:     security.Principal{Subject: subject, Claims: claims},
		Token:         token,
		Authenticated: true,
		Authorities:   authorities,
	}, nil
}

// Wrap returns an http.Handler that authenticates each request before delegating
// to next. This is the seam: the application hands the wrapped handler to a
// *gs.HttpServeMux, so the resource server sits in front of any framework engine.
//
// A missing token yields 401 when Required (default); otherwise the request is
// passed through with no Authentication attached, deferring the decision to a
// method-level guard. An invalid token always yields 401. On success the verified
// security.Authentication is attached to the request context.
func (a *Authenticator) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			if a.cfg.Required {
				unauthorized(w, "missing bearer token")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		auth, err := a.Validate(r.Context(), token)
		if err != nil {
			unauthorized(w, "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(security.WithAuthentication(r.Context(), auth)))
	})
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header,
// returning "" when absent or malformed.
func bearerToken(r *http.Request) string {
	const prefix = "bearer "
	h := r.Header.Get("Authorization")
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// unauthorized writes a 401 with a Bearer challenge.
func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	http.Error(w, msg, http.StatusUnauthorized)
}

// audienceAccepted reports whether the token carries at least one of the wanted
// audiences.
func audienceAccepted(claims jwt.MapClaims, want []string) bool {
	aud, err := claims.GetAudience()
	if err != nil {
		return false
	}
	for _, got := range aud {
		if slices.Contains(want, got) {
			return true
		}
	}
	return false
}

// claimStrings normalizes a scope/role claim into a slice. It accepts a
// space-delimited string (OAuth2 "scope") or a JSON array of strings.
func claimStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return strings.Fields(t)
	case []string:
		return t
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
