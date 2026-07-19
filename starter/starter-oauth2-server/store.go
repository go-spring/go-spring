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
	"crypto/rand"
	"encoding/base64"
	"sync"
)

// authCode is a pending authorization code: it captures what /authorize granted
// so /token can redeem it once. The PKCE challenge is carried across the
// redirect and matched against the verifier the client reveals at redemption.
type authCode struct {
	clientID    string
	redirectURI string
	scopes      []string
	subject     string
	authorities []string
	challenge   string
	method      string
	expiry      int64 // unix seconds
}

// refreshRecord backs a refresh token: it lets /token mint a fresh access token
// for the same subject and (a subset of) the originally granted scopes.
type refreshRecord struct {
	clientID    string
	subject     string
	scopes      []string
	authorities []string
	expiry      int64 // unix seconds
}

// store holds the authorization codes and refresh tokens in process memory. It
// is intentionally in-memory (single-node): a multi-node deployment would back
// this with a shared store, but that is out of scope for this starter. Because
// expiry is checked lazily on read and a cheap sweep runs on write, the store
// owns no background goroutine and therefore needs no destroy hook.
type store struct {
	mu      sync.Mutex
	codes   map[string]authCode
	refresh map[string]refreshRecord
}

func newStore() *store {
	return &store{
		codes:   map[string]authCode{},
		refresh: map[string]refreshRecord{},
	}
}

// opaqueToken returns a 32-byte URL-safe random string used as an authorization
// code or refresh token value.
func opaqueToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// putCode stores c under a freshly generated code and returns it.
func (s *store) putCode(c authCode) string {
	code := opaqueToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.codes[code] = c
	return code
}

// takeCode atomically returns and removes the code (single use). The bool
// reports whether a live, unexpired code was found.
func (s *store) takeCode(code string, nowSec int64) (authCode, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.codes[code]
	if !ok {
		return authCode{}, false
	}
	delete(s.codes, code)
	if c.expiry < nowSec {
		return authCode{}, false
	}
	return c, true
}

// putRefresh stores r under a freshly generated refresh token and returns it.
func (s *store) putRefresh(r refreshRecord) string {
	tok := opaqueToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.refresh[tok] = r
	return tok
}

// takeRefresh atomically returns and removes the refresh token (rotation on
// use). The bool reports whether a live, unexpired token was found.
func (s *store) takeRefresh(tok string, nowSec int64) (refreshRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.refresh[tok]
	if !ok {
		return refreshRecord{}, false
	}
	delete(s.refresh, tok)
	if r.expiry < nowSec {
		return refreshRecord{}, false
	}
	return r, true
}

// sweepLocked drops expired entries; it runs opportunistically on each write so
// abandoned codes/tokens do not accumulate without a background janitor. The
// caller must hold s.mu.
func (s *store) sweepLocked() {
	nowSec := now().Unix()
	for k, v := range s.codes {
		if v.expiry < nowSec {
			delete(s.codes, k)
		}
	}
	for k, v := range s.refresh {
		if v.expiry < nowSec {
			delete(s.refresh, k)
		}
	}
}
