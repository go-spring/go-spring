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
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// UserAuthFunc authenticates the end-user at the authorization endpoint and, on
// success, returns the subject to stamp on the token and the authorities to
// grant. It is the seam through which an application plugs its own login /
// session mechanism: this starter deliberately ships no user store (per the
// task's "no full IdP" scope). When unset, /authorize refuses the flow, since
// there is no one to authorize.
type UserAuthFunc func(r *http.Request) (subject string, authorities []string, ok bool)

// AuthServer is the OAuth2/OIDC authorization server. It is a Contributor-form
// bean: it owns no listener. The application mounts Handler() (or the individual
// handlers) onto the HTTP server it already runs, so the token endpoints sit
// alongside its business routes. Tokens it issues are verified by a
// starter-security-jwt Authenticator sharing the same key material.
type AuthServer struct {
	cfg    Config
	signer *signer
	store  *store

	// UserAuthFunc is set by the application to authenticate the resource owner
	// at /authorize. Leaving it nil disables the authorization_code flow.
	UserAuthFunc UserAuthFunc
}

// newAuthServer builds the server, failing fast on an ambiguous or unparsable
// signing key.
func newAuthServer(c Config) (*AuthServer, error) {
	sgn, err := newSigner(c)
	if err != nil {
		return nil, err
	}
	return &AuthServer{cfg: c, signer: sgn, store: newStore()}, nil
}

// Handler returns a mux serving the three OAuth2/OIDC endpoints — /authorize,
// /token and /jwks — which the application mounts where it likes (commonly under
// an /oauth2 prefix via http.StripPrefix).
func (s *AuthServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/authorize", s.handleAuthorize)
	mux.HandleFunc("/token", s.handleToken)
	mux.HandleFunc("/jwks", s.handleJWKS)
	return mux
}

// handleJWKS publishes the verification key set. For an HMAC signing key the set
// is empty (there is no public key to publish); the resource server shares the
// secret out of band instead.
func (s *AuthServer) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(s.signer.jwks)
}

// handleAuthorize implements the authorization endpoint of the
// authorization_code grant. It validates the client and redirect_uri, enforces
// PKCE for public clients, authenticates the resource owner via UserAuthFunc,
// and redirects back to the client with a single-use code.
func (s *AuthServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")

	client, ok := s.cfg.Clients[clientID]
	if !ok {
		http.Error(w, "unknown client_id", http.StatusBadRequest)
		return
	}
	// The redirect_uri must be validated against the allow-list before it is
	// used as a redirect target, otherwise it becomes an open redirect.
	if !slices.Contains(client.RedirectURIs, redirectURI) {
		http.Error(w, "redirect_uri not registered for client", http.StatusBadRequest)
		return
	}
	if !client.grantAllowed(grantAuthCode) {
		redirectError(w, r, redirectURI, "unauthorized_client", q.Get("state"))
		return
	}
	if q.Get("response_type") != "code" {
		redirectError(w, r, redirectURI, "unsupported_response_type", q.Get("state"))
		return
	}

	challenge := q.Get("code_challenge")
	method := q.Get("code_challenge_method")
	if challenge == "" && client.Public {
		redirectError(w, r, redirectURI, "invalid_request", q.Get("state"))
		return
	}
	if challenge != "" && method != "" && method != methodPlain && method != methodS256 {
		redirectError(w, r, redirectURI, "invalid_request", q.Get("state"))
		return
	}

	scopes, err := grantedScopes(q.Get("scope"), client)
	if err != nil {
		redirectError(w, r, redirectURI, "invalid_scope", q.Get("state"))
		return
	}

	if s.UserAuthFunc == nil {
		http.Error(w, "authorization_code flow not enabled (no UserAuthFunc)", http.StatusServiceUnavailable)
		return
	}
	subject, authorities, ok := s.UserAuthFunc(r)
	if !ok {
		http.Error(w, "resource owner not authenticated", http.StatusUnauthorized)
		return
	}

	code := s.store.putCode(authCode{
		clientID:    clientID,
		redirectURI: redirectURI,
		scopes:      scopes,
		subject:     subject,
		authorities: authorities,
		challenge:   challenge,
		method:      method,
		expiry:      now().Add(s.cfg.CodeTTL).Unix(),
	})

	u, _ := url.Parse(redirectURI)
	rq := u.Query()
	rq.Set("code", code)
	if state := q.Get("state"); state != "" {
		rq.Set("state", state)
	}
	u.RawQuery = rq.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// handleToken implements the token endpoint for all three supported grants.
func (s *AuthServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		tokenError(w, http.StatusMethodNotAllowed, "invalid_request", "POST required")
		return
	}
	if err := r.ParseForm(); err != nil {
		tokenError(w, http.StatusBadRequest, "invalid_request", "malformed form")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case grantAuthCode:
		s.tokenAuthCode(w, r)
	case grantClientCreds:
		s.tokenClientCredentials(w, r)
	case grantRefresh:
		s.tokenRefresh(w, r)
	default:
		tokenError(w, http.StatusBadRequest, "unsupported_grant_type", "unknown grant_type")
	}
}

// tokenAuthCode redeems an authorization code, enforcing client identity,
// redirect_uri equality, and the PKCE verifier.
func (s *AuthServer) tokenAuthCode(w http.ResponseWriter, r *http.Request) {
	clientID, secret, _ := clientCredentials(r)
	client, ok := s.cfg.Clients[clientID]
	if !ok || !s.clientAuthenticated(client, secret) {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	if !client.grantAllowed(grantAuthCode) {
		tokenError(w, http.StatusBadRequest, "unauthorized_client", "grant not allowed for client")
		return
	}

	code := r.PostForm.Get("code")
	rec, ok := s.store.takeCode(code, now().Unix())
	if !ok {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "code invalid or expired")
		return
	}
	if rec.clientID != clientID || rec.redirectURI != r.PostForm.Get("redirect_uri") {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "code does not match client/redirect_uri")
		return
	}
	// PKCE: required whenever a challenge was captured at /authorize (always the
	// case for public clients).
	if rec.challenge != "" {
		if !verifyPKCE(r.PostForm.Get("code_verifier"), rec.challenge, rec.method) {
			tokenError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
			return
		}
	}

	s.issueTokens(w, client, clientID, rec.subject, rec.scopes, rec.authorities, true)
}

// tokenClientCredentials issues an access token to a confidential client acting
// on its own behalf (no resource owner, so no refresh token).
func (s *AuthServer) tokenClientCredentials(w http.ResponseWriter, r *http.Request) {
	clientID, secret, _ := clientCredentials(r)
	client, ok := s.cfg.Clients[clientID]
	if !ok || client.Public || !s.clientAuthenticated(client, secret) {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	if !client.grantAllowed(grantClientCreds) {
		tokenError(w, http.StatusBadRequest, "unauthorized_client", "grant not allowed for client")
		return
	}
	scopes, err := grantedScopes(r.PostForm.Get("scope"), client)
	if err != nil {
		tokenError(w, http.StatusBadRequest, "invalid_scope", "requested scope not allowed")
		return
	}
	// The client is its own principal; scopes double as authorities.
	s.issueTokens(w, client, clientID, clientID, scopes, scopes, false)
}

// tokenRefresh rotates a refresh token and mints a fresh access token.
func (s *AuthServer) tokenRefresh(w http.ResponseWriter, r *http.Request) {
	clientID, secret, _ := clientCredentials(r)
	client, ok := s.cfg.Clients[clientID]
	if !ok || !s.clientAuthenticated(client, secret) {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	if !client.grantAllowed(grantRefresh) {
		tokenError(w, http.StatusBadRequest, "unauthorized_client", "grant not allowed for client")
		return
	}
	rec, ok := s.store.takeRefresh(r.PostForm.Get("refresh_token"), now().Unix())
	if !ok || rec.clientID != clientID {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "refresh_token invalid or expired")
		return
	}
	// A refresh may request a narrower scope; absent, the original set is kept.
	scopes := rec.scopes
	if req := r.PostForm.Get("scope"); req != "" {
		narrowed, err := narrowScopes(req, rec.scopes)
		if err != nil {
			tokenError(w, http.StatusBadRequest, "invalid_scope", "scope exceeds original grant")
			return
		}
		scopes = narrowed
	}
	s.issueTokens(w, client, clientID, rec.subject, scopes, rec.authorities, true)
}

// issueTokens signs an access token and, when withRefresh is set, mints a
// refresh token, then writes the standard token response.
func (s *AuthServer) issueTokens(w http.ResponseWriter, _ ClientConfig, clientID, subject string, scopes, authorities []string, withRefresh bool) {
	iat := now()
	exp := iat.Add(s.cfg.AccessTokenTTL)
	claims := jwt.MapClaims{
		"sub":       subject,
		"client_id": clientID,
		"iat":       iat.Unix(),
		"exp":       exp.Unix(),
		"jti":       opaqueToken(),
	}
	if s.cfg.Issuer != "" {
		claims["iss"] = s.cfg.Issuer
	}
	if len(scopes) > 0 {
		claims["scope"] = strings.Join(scopes, " ")
	}
	// Carry any extra authorities (roles) not already expressed as scopes.
	if roles := extra(authorities, scopes); len(roles) > 0 {
		claims["roles"] = roles
	}

	access, err := s.signer.sign(claims)
	if err != nil {
		tokenError(w, http.StatusInternalServerError, "server_error", "failed to sign token")
		return
	}

	resp := tokenResponse{
		AccessToken: access,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.cfg.AccessTokenTTL.Seconds()),
		Scope:       strings.Join(scopes, " "),
	}
	if withRefresh {
		resp.RefreshToken = s.store.putRefresh(refreshRecord{
			clientID:    clientID,
			subject:     subject,
			scopes:      scopes,
			authorities: authorities,
			expiry:      now().Add(s.cfg.RefreshTokenTTL).Unix(),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// clientAuthenticated verifies the presented secret for a client. A public
// client authenticates by identity alone (it has no secret); a confidential
// client's secret is compared in constant time.
func (s *AuthServer) clientAuthenticated(client ClientConfig, secret string) bool {
	if client.Public {
		return true
	}
	if client.Secret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(client.Secret), []byte(secret)) == 1
}

// grant types accepted at the token endpoint.
const (
	grantAuthCode    = "authorization_code"
	grantClientCreds = "client_credentials"
	grantRefresh     = "refresh_token"
)

// grantAllowed reports whether the client may use the grant; an empty allow-list
// permits all supported grants.
func (c ClientConfig) grantAllowed(grant string) bool {
	if len(c.GrantTypes) == 0 {
		return true
	}
	return slices.Contains(c.GrantTypes, grant)
}

// clientCredentials extracts the client_id/secret from HTTP Basic auth, falling
// back to the form body (client_id / client_secret) for public clients and
// bodies that carry them inline.
func clientCredentials(r *http.Request) (id, secret string, ok bool) {
	if id, secret, ok = r.BasicAuth(); ok {
		return id, secret, true
	}
	return r.PostForm.Get("client_id"), r.PostForm.Get("client_secret"), false
}

// grantedScopes parses a space-delimited scope request and rejects any scope
// outside the client's allow-list. An empty allow-list imposes no restriction.
func grantedScopes(req string, client ClientConfig) ([]string, error) {
	scopes := strings.Fields(req)
	if len(client.Scopes) == 0 {
		return scopes, nil
	}
	for _, s := range scopes {
		if !slices.Contains(client.Scopes, s) {
			return nil, errBadScope
		}
	}
	return scopes, nil
}

// narrowScopes ensures every requested scope is within the original grant.
func narrowScopes(req string, original []string) ([]string, error) {
	scopes := strings.Fields(req)
	for _, s := range scopes {
		if !slices.Contains(original, s) {
			return nil, errBadScope
		}
	}
	return scopes, nil
}

// extra returns the authorities not already present in scopes, so a role that
// is not also a scope is still carried on the token.
func extra(authorities, scopes []string) []string {
	var out []string
	for _, a := range authorities {
		if !slices.Contains(scopes, a) {
			out = append(out, a)
		}
	}
	return out
}

// tokenResponse is the RFC 6749 §5.1 successful token response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// errBadScope flags a requested scope outside what the client (or the original
// grant) allows.
var errBadScope = &oauthError{code: "invalid_scope"}

type oauthError struct{ code string }

func (e *oauthError) Error() string { return e.code }

// tokenError writes an RFC 6749 §5.2 error response.
func tokenError(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

// redirectError sends the OAuth2 error back to the client's redirect_uri, which
// has already been validated against the allow-list.
func redirectError(w http.ResponseWriter, r *http.Request, redirectURI, code, state string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, code, http.StatusBadRequest)
		return
	}
	q := u.Query()
	q.Set("error", code)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// writeJSON serializes v as JSON with no-store caching, as token responses must
// not be cached.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
