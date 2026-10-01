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

package security

import (
	"strings"
)

// ParseBearerToken extracts the credential from an "Authorization: Bearer
// <token>" header value, returning "" when the value is absent or not a bearer
// credential. The HTTP-layer middlewares in the server starters (stdlib, gin,
// echo) all parse through this function so the acceptance rules cannot drift
// between families.
func ParseBearerToken(authorizationHeader string) string {
	const prefix = "bearer "
	if len(authorizationHeader) < len(prefix) || !strings.EqualFold(authorizationHeader[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(authorizationHeader[len(prefix):])
}

// RFC 6750 error codes for a rejected bearer credential. Pass one of these to
// [BearerChallenge]; they are the wire values a client matches on, so they must
// not be spelled out at each call site.
const (
	// BearerErrorInvalidRequest is for a request that was malformed and could not
	// carry a usable credential.
	BearerErrorInvalidRequest = "invalid_request"

	// BearerErrorInvalidToken is for a credential that was presented but
	// rejected.
	BearerErrorInvalidToken = "invalid_token"

	// BearerErrorInsufficientScope is for a caller whose credential is valid but
	// lacks the authority the request needs. It accompanies a 403.
	BearerErrorInsufficientScope = "insufficient_scope"
)

// BearerChallenge returns the WWW-Authenticate header value a server sends when
// it rejects a bearer credential, so every server family answers the same way.
//
// errCode is one of the BearerError* constants, or "" when the request carried
// no credential at all — RFC 6750 then requires the bare scheme, because an
// error code describes a credential that was present and rejected.
func BearerChallenge(errCode string) string {
	if errCode == "" {
		return "Bearer"
	}
	return `Bearer error="` + errCode + `"`
}
