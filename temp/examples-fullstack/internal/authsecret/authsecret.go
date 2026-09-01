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

// Package authsecret holds the single HMAC secret shared by the whole demo. In a
// real system the resource server would verify against an identity provider's
// public key or a JWKS endpoint; here every process (the token minter and the
// order resource server) reads the same constant so the sample is self-contained
// with no external IdP. It matches spring.security.jwt.api.secret in
// cmd/order/conf/app.properties.
package authsecret

// Secret is the HS256 signing/verification key. Keep it in one place so the
// minted token and the order service's verifier can never drift apart.
const Secret = "fullstack-shared-secret"
