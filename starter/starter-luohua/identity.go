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

package luohua

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-spring.org/cloud/security"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

// LuohuaSSO is the luohua company SSO: a security.TokenValidator backed by a
// shared HMAC secret. It is deliberately dependency-free (stdlib only) so the
// demo runs offline; a real company would swap in its OIDC/JWT/JWKS verifier
// behind the same security.TokenValidator seam — the middleware shells accept
// any implementer, and luohua never special-cases its own.
type LuohuaSSO struct {
	secret []byte
	issuer string
}

// claims is the unsigned body of a luohua token.
type claims struct {
	Issuer      string   `json:"iss"`
	Subject     string   `json:"sub"`
	Tenant      string   `json:"tenant,omitempty"`
	Authorities []string `json:"auth"`
	ExpiresAt   int64    `json:"exp"`
}

// NewLuohuaSSO builds a validator signing/verifying with the shared secret and
// expecting the given issuer.
func NewLuohuaSSO(secret, issuer string) *LuohuaSSO {
	return &LuohuaSSO{secret: []byte(secret), issuer: issuer}
}

// Issue mints a signed token for subject with authorities and a tenant, valid
// for ttl.
func (s *LuohuaSSO) Issue(subject, tenant string, authorities []string, ttl time.Duration) (string, error) {
	body, err := json.Marshal(claims{
		Issuer:      s.issuer,
		Subject:     subject,
		Tenant:      tenant,
		Authorities: authorities,
		ExpiresAt:   time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(body)
	return s.sign(payload), nil
}

func (s *LuohuaSSO) sign(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Validate verifies a luohua token and, on success, returns the identity the
// request carried. Any failure is a plain error — the security middleware maps
// it to 401, so luohua needs no HTTP knowledge here.
func (s *LuohuaSSO) Validate(_ context.Context, token string) (*security.Authentication, error) {
	if token == "" {
		return nil, errors.New("luohua: empty token")
	}
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return nil, errors.New("luohua: malformed token")
	}

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	if !hmac.Equal([]byte(sig), []byte(base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))) {
		return nil, errors.New("luohua: bad signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("luohua: decode: %w", err)
	}
	var c claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("luohua: decode claims: %w", err)
	}
	if c.Issuer != s.issuer {
		return nil, errors.New("luohua: bad issuer")
	}
	if c.ExpiresAt != 0 && c.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("luohua: token expired")
	}

	return &security.Authentication{
		Principal: security.Principal{
			Subject: c.Subject,
			Claims:  map[string]any{"issuer": c.Issuer, "tenant": c.Tenant},
		},
		Token:        token,
		Authenticated: true,
		Authorities:  c.Authorities,
	}, nil
}

// Authority vocabulary — luohua uses a plain "permission:scope" convention the
// security.Authorize shells and Require guards compare against.
const (
	AuthorityOrdersRead  = "orders:read"
	AuthorityOrdersWrite = "orders:write"
)

func init() {
	// Armed by any spring.luohua.identity.* key; the secret is required, so a
	// half-configured identity fails startup loudly instead of silently issuing
	// nothing.
	gs.Module(gs.OnProperty("spring.luohua.identity"), func(r gs.BeanProvider, p flatten.Storage) error {
		if off, err := disabled(p); err != nil {
			return err
		} else if off {
			return nil // whole baseline off; do not assemble the keyed bean capability
		}
		var c IdentityConfig
		if err := conf.Bind(p, &c, "${spring.luohua.identity:=}"); err != nil {
			return err
		}
		sso := NewLuohuaSSO(c.Secret, c.Issuer)
		// This is a default: when an application provides its own
		// security.TokenValidator (e.g. a JWT/OIDC verifier), OnMissingBean steps
		// luohua's LuohuaSSO aside rather than competing — the same step-aside the
		// i18n default performs. Without it two TokenValidator beans make the
		// container's single-value autowire ambiguous and startup fails.
		r.Provide(func() *LuohuaSSO { return sso }).
			Condition(gs.OnMissingBean[security.TokenValidator]()).
			Export(gs.As[security.TokenValidator]()).Caller(1)
		return nil
	})
}
