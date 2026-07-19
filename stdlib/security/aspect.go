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
	"go-spring.org/stdlib/aspect"
)

// Require returns an aspect [aspect.Interceptor] that enforces method-level
// security — the @PreAuthorize equivalent, achieved through the AOP-equivalent
// interceptor chain (aspect) rather than a bytecode/annotation port.
//
// It reads the [Authentication] carried on the joinpoint context (put there by a
// resource-server middleware via [WithAuthentication]) and:
//   - returns [ErrUnauthenticated] when the caller carries no verified identity;
//   - returns [ErrForbidden] when authenticated but holding none of authorities;
//   - otherwise proceeds down the chain to the target.
//
// With no authorities it degrades to "authenticated caller required". Wire it
// into a chain alongside the other builtin interceptors:
//
//	chain := aspect.NewChain(security.Require("orders:write"))
//	_, err := aspect.Around(chain, ctx, "PlaceOrder", svc.placeOrder)
func Require(authorities ...string) aspect.Interceptor {
	return aspect.InterceptorFunc(func(jp *aspect.Joinpoint) (any, error) {
		auth, _ := FromContext(jp.Context)
		if auth == nil || !auth.Authenticated {
			return nil, ErrUnauthenticated
		}
		if !auth.HasAnyAuthority(authorities...) {
			return nil, ErrForbidden
		}
		return jp.Proceed(jp.Context)
	})
}
