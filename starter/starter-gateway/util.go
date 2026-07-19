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

package StarterGateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// routeIDContextKey carries the matched route id down the filter chain so
// filters (rate-limit, metrics) can attribute a request to its route.
type routeIDContextKey struct{}

// withRouteID returns a context carrying the matched route id.
func withRouteID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, routeIDContextKey{}, id)
}

// routeIDFromContext returns the matched route id, or "" if unset.
func routeIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(routeIDContextKey{}).(string)
	return id
}

// newRequestID returns a random 128-bit hex id used by the request-id filter
// when the inbound request carries none.
func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
