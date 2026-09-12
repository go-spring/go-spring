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
	"time"
)

// Config configures one OAuth2 resource-server JWT verification instance bound
// under spring.security.oauth2.resource.jwt.instances.<name>. Exactly one verification
// key source must be configured — an issuer URI (resolved through OIDC
// discovery to a JWKS endpoint), an asymmetric public key (PEM inline or file),
// or a shared HMAC secret. The constructor fails fast when zero or more than
// one is set,
// since a resource server that cannot decide how to verify a token is
// misconfigured.
type Config struct {
	// IssuerURI is the OAuth2/OpenID Connect issuer identifier (e.g.
	// "https://auth.example.com"). At startup the starter fetches
	// {issuer-uri}/.well-known/openid-configuration and reads its jwks_uri, so
	// no manual JWKS URL is needed. When set, it is also the expected "iss"
	// claim unless issuer overrides it.
	IssuerURI string `value:"${issuer-uri:=}"`

	// Issuer, when set, overrides the expected "iss" claim (defaulting to
	// issuer-uri). Empty disables issuer checking only when issuer-uri is empty.
	Issuer string `value:"${issuer:=}"`

	// PublicKey is an inline PEM-encoded RSA or ECDSA public key for RS*/ES*/PS*
	// verification; an alternative to issuer-uri for statically provisioned keys.
	PublicKey string `value:"${public-key:=}"`

	// PublicKeyFile is the path to a PEM-encoded public key file.
	PublicKeyFile string `value:"${public-key-file:=}"`

	// Secret is the shared HMAC secret for HS256/HS384/HS512 verification.
	Secret string `value:"${secret:=}"`

	// Audiences, when non-empty, is the set of acceptable "aud" values; a token
	// is accepted if it carries at least one of them. Empty disables audience
	// checking.
	Audiences []string `value:"${audiences:=}"`

	// Algorithm, when set, pins the single accepted signing algorithm (e.g.
	// "RS256", "HS256", "ES256"). Empty accepts any algorithm compatible with
	// the configured key source, which prevents algorithm-confusion downgrades.
	Algorithm string `value:"${algorithm:=}"`

	// JWKSRefresh bounds how long a fetched JWKS is cached before a refresh; a
	// token whose "kid" is unknown also triggers an immediate refresh.
	JWKSRefresh time.Duration `value:"${jwks-refresh:=15m}"`

	// JWKSTimeout bounds each discovery/JWKS HTTP fetch.
	JWKSTimeout time.Duration `value:"${jwks-timeout:=10s}"`

	// ScopeClaim is the claim carrying granted scopes; its value may be a
	// space-delimited string (OAuth2) or a JSON array. Default "scope".
	ScopeClaim string `value:"${scope-claim:=scope}"`

	// RolesClaim is the claim carrying granted roles; its value may be a
	// space-delimited string or a JSON array. Default "roles".
	RolesClaim string `value:"${roles-claim:=roles}"`

	// Leeway is the clock-skew tolerance applied to exp/nbf/iat validation.
	Leeway time.Duration `value:"${leeway:=0}"`
}

// keySource identifies which verification material a Config selects.
type keySource int

const (
	sourceNone keySource = iota
	sourceHMAC
	sourcePEM
	sourceIssuer
)

// source reports the single configured key source, or an error when zero or
// more than one is set.
func (c Config) source() (keySource, error) {
	var sources []keySource
	if c.Secret != "" {
		sources = append(sources, sourceHMAC)
	}
	if c.PublicKey != "" || c.PublicKeyFile != "" {
		sources = append(sources, sourcePEM)
	}
	if c.IssuerURI != "" {
		sources = append(sources, sourceIssuer)
	}
	switch len(sources) {
	case 0:
		return sourceNone, errNoKeySource
	case 1:
		return sources[0], nil
	default:
		return sourceNone, errMultipleKeySources
	}
}

// issuer returns the expected "iss" claim: an explicit issuer if set, else the
// issuer URI (per OIDC, the issuer identifier equals the issuer URI), else "".
func (c Config) expectedIssuer() string {
	if c.Issuer != "" {
		return c.Issuer
	}
	return c.IssuerURI
}
