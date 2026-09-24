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

// Package randutil generates random identifier strings: the fencing tokens,
// request ids, session ids, CSRF tokens and opaque OAuth2 values that litter
// a codebase with hand-rolled crypto/rand snippets otherwise.
package randutil

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

// Hex returns 2n hex characters from n random bytes (n bytes of entropy as
// 128 bits when n=16, the common short-id size). Since Go 1.24 crypto/rand
// never fails, so neither does this.
func Hex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// URLSafe returns the unpadded base64url encoding of n random bytes (4n/3
// characters). n=32 (256 bits of entropy) is the common unguessable-token
// size. Since Go 1.24 crypto/rand never fails, so neither does this.
func URLSafe(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
