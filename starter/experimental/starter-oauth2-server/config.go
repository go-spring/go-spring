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
	"time"
)

// Config binds ${spring.oauth2.server}: the authorization server's token
// lifetimes, its single signing key, and the set of registered clients. The
// signing key is shared with the resource server by configuration (the same
// HMAC secret, or the public half of the asymmetric key / the /jwks endpoint),
// so tokens this server issues verify against a starter-security-jwt
// Authenticator — that is the "共用密钥体系" the two halves rely on.
//
// Exactly one signing source must be set — an HMAC secret or a PEM private key
// (inline or file). The constructor fails fast when zero or both are configured,
// since a server that cannot decide how to sign is misconfigured.
type Config struct {
	// Issuer is the "iss" claim stamped on every issued token; when the resource
	// server pins an issuer, the two must agree. Empty omits the claim.
	Issuer string `value:"${issuer:=}"`

	// Algorithm optionally pins the signing algorithm (e.g. "RS256", "HS256",
	// "ES256"). Empty picks a sensible default for the key source: HS256 for an
	// HMAC secret, RS256 for an RSA key, ES256 for an EC key.
	Algorithm string `value:"${algorithm:=}"`

	// Secret is the shared HMAC secret used to sign (and, by the resource server,
	// to verify) HS256/384/512 tokens. Mutually exclusive with the PEM key.
	Secret string `value:"${secret:=}"`

	// PrivateKey is an inline PEM-encoded RSA or ECDSA private key for
	// RS*/ES*/PS* signing. Its public half is published at /jwks.
	PrivateKey string `value:"${private-key:=}"`

	// PrivateKeyFile is the path to a PEM private key file; an alternative to
	// PrivateKey.
	PrivateKeyFile string `value:"${private-key-file:=}"`

	// KeyID is the "kid" stamped on the token header and published in the JWKS,
	// so a rotating resource server can select the right key. Empty uses "default".
	KeyID string `value:"${key-id:=default}"`

	// AccessTokenTTL is the lifetime of an issued access token.
	AccessTokenTTL time.Duration `value:"${access-token-ttl:=1h}"`

	// RefreshTokenTTL is the lifetime of an issued refresh token.
	RefreshTokenTTL time.Duration `value:"${refresh-token-ttl:=24h}"`

	// CodeTTL is the lifetime of an authorization code; it should be short since
	// the code is redeemed immediately at the token endpoint.
	CodeTTL time.Duration `value:"${code-ttl:=1m}"`

	// Clients maps a client_id to its registration. An unregistered client_id is
	// rejected at both /authorize and /token.
	Clients map[string]ClientConfig `value:"${clients:=}"`
}

// ClientConfig registers one OAuth2 client. A confidential client authenticates
// at the token endpoint with Secret; a public client (a SPA or native app that
// cannot keep a secret) sets Public=true, carries no secret, and must use PKCE.
type ClientConfig struct {
	// Secret is the confidential client's credential, checked in constant time at
	// the token endpoint. Empty for a public client.
	Secret string `value:"${secret:=}"`

	// Public marks a client that cannot hold a secret. PKCE is then mandatory on
	// the authorization_code flow, since the code is the only thing binding the
	// redirect to the token request.
	Public bool `value:"${public:=false}"`

	// RedirectURIs is the allow-list of exact redirect targets for the
	// authorization_code flow. A request whose redirect_uri is not listed is
	// rejected. Required for authorization_code clients.
	RedirectURIs []string `value:"${redirect-uris:=}"`

	// Scopes is the set of scopes this client may be granted; a request for a
	// scope outside the set is rejected. Empty means "no scope restriction".
	Scopes []string `value:"${scopes:=}"`

	// GrantTypes is the allow-list of grants this client may use
	// ("authorization_code", "client_credentials", "refresh_token"). Empty allows
	// all three.
	GrantTypes []string `value:"${grant-types:=}"`
}
