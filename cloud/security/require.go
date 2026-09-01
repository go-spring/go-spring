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
	"context"
)

// Require returns the @PreAuthorize equivalent as a plain decorator: it
// enforces method-level security by checking the caller's identity before the
// wrapped business logic runs.
//
// It reads the [Authentication] carried on the context (put there by a
// resource-server middleware via [WithAuthentication]) and:
//   - returns [ErrUnauthenticated] when the caller carries no verified identity;
//   - returns [ErrForbidden] when authenticated but holding none of authorities;
//   - otherwise calls proceed.
//
// With no authorities it degrades to "authenticated caller required". There is
// deliberately no shared interceptor-chain protocol: the decorator is an
// ordinary function, and combining cross-cutting concerns is ordinary nesting —
//
//	err := Require("orders:write")(ctx, func(ctx context.Context) error {
//	    return transaction.GlobalTransactional(coord, reg)(ctx, "OrderService.Place", place)
//	})
func Require(authorities ...string) func(ctx context.Context, proceed func(context.Context) error) error {
	return func(ctx context.Context, proceed func(context.Context) error) error {
		auth, _ := FromContext(ctx)
		if auth == nil || !auth.Authenticated {
			return ErrUnauthenticated
		}
		if !auth.HasAnyAuthority(authorities...) {
			return ErrForbidden
		}
		return proceed(ctx)
	}
}
