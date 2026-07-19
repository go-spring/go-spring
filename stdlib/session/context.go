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

package session

import "context"

// ctxKey is an unexported context key type so the stored Session cannot collide
// with keys from other packages.
type ctxKey struct{}

// WithSession returns a copy of ctx carrying s. The Manager middleware calls it
// after loading (or creating) the session so downstream handlers can read it via
// [FromContext].
func WithSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, ctxKey{}, s)
}

// FromContext returns the [Session] carried by ctx. The boolean reports whether
// a session was attached at all; a handler running behind [Manager.Middleware]
// can rely on it being present.
func FromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(ctxKey{}).(*Session)
	return s, ok
}
