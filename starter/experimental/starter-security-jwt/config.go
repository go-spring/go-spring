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
	"time"
)

// Config defines a JWT resource-server verification configuration. Each instance
// yields an *Authenticator that verifies bearer tokens and, on success, attaches
// a security.Authentication to the request context.
//
// Exactly one verification key source must be configured — a shared HMAC secret,
// an asymmetric public key (PEM inline or file), or a remote JWKS endpoint. The
// constructor fails fast when zero or more than one is set, since a resource
// server that cannot decide how to verify a token is misconfigured.
type Config struct {
	// Issuer, when set, is the expected "iss" claim; tokens with a different
	// issuer are rejected. Empty disables issuer checking.
	Issuer string `value:"${issuer:=}"`

	// Audience, when non-empty, is the set of acceptable "aud" values; a token is
	// accepted if it carries at least one of them. Empty disables audience checking.
	Audience []string `value:"${audience:=}"`

	// Algorithm, when set, pins the single accepted signing algorithm (e.g.
	// "RS256", "HS256", "ES256"). Empty accepts any algorithm compatible with the
	// configured key source (never HMAC for an asymmetric source), which prevents
	// algorithm-confusion downgrades.
	Algorithm string `value:"${algorithm:=}"`

	// Secret is the shared HMAC secret for HS256/HS384/HS512 verification.
	Secret string `value:"${secret:=}"`

	// PublicKey is an inline PEM-encoded RSA or ECDSA public key for RS*/ES*/PS*
	// verification.
	PublicKey string `value:"${public-key:=}"`

	// PublicKeyFile is the path to a PEM-encoded public key file; an alternative
	// to PublicKey.
	PublicKeyFile string `value:"${public-key-file:=}"`

	// JWKSURL is a remote JWKS (JSON Web Key Set) endpoint; keys are fetched at
	// startup and refreshed, and each token's "kid" header selects the key.
	JWKSURL string `value:"${jwks-url:=}"`

	// JWKSRefresh bounds how long a fetched JWKS is cached before a refresh; a
	// token whose "kid" is unknown also triggers an immediate refresh.
	JWKSRefresh time.Duration `value:"${jwks-refresh:=15m}"`

	// JWKSTimeout bounds each JWKS HTTP fetch.
	JWKSTimeout time.Duration `value:"${jwks-timeout:=10s}"`

	// ScopeClaim is the claim carrying granted scopes; its value may be a
	// space-delimited string (OAuth2) or a JSON array. Default "scope".
	ScopeClaim string `value:"${scope-claim:=scope}"`

	// RolesClaim is the claim carrying granted roles; its value may be a
	// space-delimited string or a JSON array. Default "roles".
	RolesClaim string `value:"${roles-claim:=roles}"`

	// Leeway is the clock-skew tolerance applied to exp/nbf/iat validation.
	Leeway time.Duration `value:"${leeway:=0}"`

	// Required controls what happens when a request carries no bearer token:
	// true (default) rejects with 401; false lets the request through with no
	// Authentication attached, so downstream method-level guards decide.
	Required bool `value:"${required:=true}"`
}

// keySource identifies which verification material a Config selects.
type keySource int

const (
	sourceNone keySource = iota
	sourceHMAC
	sourcePEM
	sourceJWKS
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
	if c.JWKSURL != "" {
		sources = append(sources, sourceJWKS)
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
