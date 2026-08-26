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

// Package StarterOauth2ResourceServer turns an application into an OAuth2
// resource server: it verifies incoming JWT bearer tokens against a trusted
// issuer and exposes the result as the framework-neutral
// security.TokenValidator seam.
package StarterOauth2ResourceServer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go-spring.org/cloud/experimental/security"
	"go-spring.org/stdlib/errutil"
)

var (
	errNoKeySource        = errors.New("oauth2-resource-server: no verification key source configured (set one of issuer-uri, public-key/public-key-file, secret)")
	errMultipleKeySources = errors.New("oauth2-resource-server: multiple verification key sources configured (set exactly one of issuer-uri, public-key/public-key-file, secret)")
	errAudience           = errors.New("oauth2-resource-server: token audience not accepted")
)

// Validator verifies JWT bearer tokens issued by an OAuth2 authorization
// server. It implements security.TokenValidator and is contributed as a named
// bean under spring.security.oauth2.resource.jwt.<name>; the application
// composes it with security.Authenticate(v, required) (or any consumer of the
// TokenValidator seam) to protect its endpoints.
type Validator struct {
	cfg     Config
	parser  *jwt.Parser
	keyfunc jwt.Keyfunc
	jwks    *jwksCache // non-nil only for the issuer-uri source
}

// newValidator builds a Validator from its configuration, failing fast when the
// key source is ambiguous, the PEM key cannot be parsed, or the issuer's
// discovery/JWKS endpoints cannot be reached at startup.
func newValidator(c Config) (*Validator, error) {
	src, err := c.source()
	if err != nil {
		return nil, err
	}
	methods, err := validMethods(c, src)
	if err != nil {
		return nil, err
	}

	v := &Validator{cfg: c}

	switch src {
	case sourceHMAC:
		secret := []byte(c.Secret)
		v.keyfunc = func(*jwt.Token) (any, error) { return secret, nil }
	case sourcePEM:
		key, err := parsePEMPublicKey(c)
		if err != nil {
			return nil, err
		}
		v.keyfunc = func(*jwt.Token) (any, error) { return key, nil }
	case sourceIssuer:
		uri, err := discoverJWKSURI(c)
		if err != nil {
			return nil, err
		}
		cache, err := newJWKSCache(uri, c.JWKSRefresh, c.JWKSTimeout)
		if err != nil {
			return nil, err
		}
		v.jwks = cache
		v.keyfunc = func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			return cache.key(kid)
		}
	}

	opts := []jwt.ParserOption{
		jwt.WithValidMethods(methods),
		jwt.WithLeeway(c.Leeway),
	}
	if iss := c.expectedIssuer(); iss != "" {
		opts = append(opts, jwt.WithIssuer(iss))
	}
	v.parser = jwt.NewParser(opts...)
	return v, nil
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
	return nil, fmt.Errorf("oauth2-resource-server: algorithm %q is not compatible with the configured key source", c.Algorithm)
}

// discoverJWKSURI performs OIDC discovery: it fetches
// {issuer-uri}/.well-known/openid-configuration and returns the advertised
// jwks_uri. Trailing slashes on the issuer URI are tolerated.
func discoverJWKSURI(c Config) (string, error) {
	base := strings.TrimSuffix(c.IssuerURI, "/")
	url := base + "/.well-known/openid-configuration"

	client := &http.Client{Timeout: c.JWKSTimeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", errutil.Explain(err, "oauth2-resource-server: build discovery request")
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errutil.Explain(err, "oauth2-resource-server: fetch discovery document %s", url)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth2-resource-server: fetch discovery document %s: status %d", url, resp.StatusCode)
	}

	var doc struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", errutil.Explain(err, "oauth2-resource-server: decode discovery document %s", url)
	}
	if doc.JWKSURI == "" {
		return "", fmt.Errorf("oauth2-resource-server: discovery document %s has no jwks_uri", url)
	}
	return doc.JWKSURI, nil
}

// parsePEMPublicKey loads the PEM public key from the inline value or the file,
// trying RSA then ECDSA.
func parsePEMPublicKey(c Config) (any, error) {
	pem := []byte(c.PublicKey)
	if c.PublicKeyFile != "" {
		b, err := os.ReadFile(c.PublicKeyFile)
		if err != nil {
			return nil, errutil.Explain(err, "oauth2-resource-server: read public-key-file %s", c.PublicKeyFile)
		}
		pem = b
	}
	if rsaKey, err := jwt.ParseRSAPublicKeyFromPEM(pem); err == nil {
		return rsaKey, nil
	}
	if ecKey, err := jwt.ParseECPublicKeyFromPEM(pem); err == nil {
		return ecKey, nil
	}
	return nil, errors.New("oauth2-resource-server: public key is neither a valid RSA nor ECDSA PEM")
}

// Validate verifies a raw token string and returns the security.Authentication
// it represents. It implements security.TokenValidator. Signature (RS256/HS256/
// ES256/...), exp, nbf and — when configured — iss and aud are all checked by
// the parser; any failure is an error, never an unauthenticated result.
func (v *Validator) Validate(_ context.Context, token string) (*security.Authentication, error) {
	claims := jwt.MapClaims{}
	tok, err := v.parser.ParseWithClaims(token, claims, v.keyfunc)
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errutil.Explain(security.ErrUnauthenticated, "oauth2-resource-server: invalid token")
	}
	if len(v.cfg.Audiences) > 0 && !audienceAccepted(claims, v.cfg.Audiences) {
		return nil, errAudience
	}

	subject, _ := claims["sub"].(string)
	authorities := append(claimStrings(claims[v.cfg.ScopeClaim]), claimStrings(claims[v.cfg.RolesClaim])...)

	return &security.Authentication{
		Principal:     security.Principal{Subject: subject, Claims: claims},
		Token:         token,
		Authenticated: true,
		Authorities:   authorities,
	}, nil
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
