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
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// PKCE (RFC 7636) binds an authorization_code to the client that started the
// flow: the client keeps a secret verifier, sends only its hash (the challenge)
// to /authorize, and reveals the verifier at /token. An attacker who intercepts
// the code cannot redeem it without the verifier. This starter enforces it for
// public clients, where the code is otherwise the only thing protecting the
// exchange.

// GenerateVerifier returns a high-entropy code_verifier (43 chars,
// base64url-encoded 32 random bytes) suitable for a client to hold across the
// redirect. It is exported so the example (and real clients) can drive the flow
// without pulling in a separate OAuth2 client library.
func GenerateVerifier() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Challenge derives the code_challenge from a verifier under the given method:
// "S256" (recommended) is BASE64URL(SHA256(verifier)); "plain" (and the empty
// default) is the verifier itself. An unknown method yields "".
func Challenge(verifier, method string) string {
	switch method {
	case "", methodPlain:
		return verifier
	case methodS256:
		sum := sha256.Sum256([]byte(verifier))
		return base64.RawURLEncoding.EncodeToString(sum[:])
	default:
		return ""
	}
}

const (
	methodPlain = "plain"
	methodS256  = "S256"
)

// verifyPKCE reports whether verifier matches the stored challenge under method,
// comparing in constant time. A stored challenge with an unsupported method
// never matches.
func verifyPKCE(verifier, challenge, method string) bool {
	want := Challenge(verifier, method)
	if want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(challenge)) == 1
}
