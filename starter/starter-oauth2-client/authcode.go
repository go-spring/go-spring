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

// This file wires the OAuth2 authorization_code (user redirect / login) grant.
// Each named entry under "${spring.oauth2.authcode}" yields an *oauth2.Config:
// callers use cfg.AuthCodeURL(state) to build the redirect URL and
// cfg.Exchange(ctx, code) to swap the returned code for a token.

package StarterOAuth2Client

import (
	"go-spring.org/spring/gs"
	"golang.org/x/oauth2"
)

func init() {
	// Register multiple OAuth2 authorization_code configurations as a group.
	// Each instance is created from the configuration under "${spring.oauth2.authcode}".
	// The resulting *oauth2.Config holds no closable resource, so no destroy callback is needed.
	gs.Group("${spring.oauth2.authcode}", newAuthCodeConfig, nil)
}

// AuthCodeConfig defines an OAuth2 authorization_code configuration. Each
// instance yields an *oauth2.Config used to drive the user redirect / login
// flow: build an authorization URL with AuthCodeURL and exchange the returned
// code for a token with Exchange.
type AuthCodeConfig struct {
	// ClientID is the OAuth2 client identifier issued by the authorization server.
	ClientID string `value:"${client-id}"`

	// ClientSecret is the OAuth2 client secret issued by the authorization server.
	ClientSecret string `value:"${client-secret}"`

	// AuthURL is the authorization endpoint the user is redirected to,
	// e.g., "https://auth.example.com/oauth/authorize".
	AuthURL string `value:"${auth-url}"`

	// TokenURL is the token endpoint that swaps an authorization code for a token,
	// e.g., "https://auth.example.com/oauth/token".
	TokenURL string `value:"${token-url}"`

	// RedirectURL is the callback URL registered with the authorization server.
	RedirectURL string `value:"${redirect-url:=}"`

	// Scopes is the optional list of scopes requested during authorization.
	Scopes []string `value:"${scopes:=}"`
}

// newAuthCodeConfig builds an *oauth2.Config for the authorization_code grant.
func newAuthCodeConfig(c AuthCodeConfig) (*oauth2.Config, error) {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Scopes:       c.Scopes,
		Endpoint:     oauth2.Endpoint{AuthURL: c.AuthURL, TokenURL: c.TokenURL},
	}, nil
}
