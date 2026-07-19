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

import "context"

// ctxKey is an unexported context key type so the stored Authentication cannot
// collide with keys from other packages.
type ctxKey struct{}

// WithAuthentication returns a copy of ctx carrying auth. A resource-server
// middleware calls it after verifying a credential so downstream handlers and
// method-level guards can read the identity via [FromContext].
func WithAuthentication(ctx context.Context, auth *Authentication) context.Context {
	return context.WithValue(ctx, ctxKey{}, auth)
}

// FromContext returns the [Authentication] carried by ctx, if any. The boolean
// reports whether an Authentication was present at all — a caller that needs to
// distinguish "no identity attached" from "attached but anonymous" can use it;
// most callers can ignore it and rely on the *Authentication methods being
// nil-safe.
func FromContext(ctx context.Context) (*Authentication, bool) {
	auth, ok := ctx.Value(ctxKey{}).(*Authentication)
	return auth, ok
}
