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

package at

import (
	"context"

	"go-spring.org/spring/aspect"
)

// xidKey is the context key carrying the global transaction id.
type xidKey struct{}

// WithXID returns a context carrying the global transaction id. The coordinator
// sets it in [Coordinator.Begin]; resource interceptors read it back with
// [XIDFromContext] to learn they are inside a global transaction and under which
// id to record undo logs and register their branch.
func WithXID(ctx context.Context, xid string) context.Context {
	return context.WithValue(ctx, xidKey{}, xid)
}

// XIDFromContext returns the global transaction id previously stored by
// [WithXID], and whether one was present. A resource interceptor uses the "not
// present" case to stay completely transparent outside a global transaction.
func XIDFromContext(ctx context.Context) (string, bool) {
	xid, ok := ctx.Value(xidKey{}).(string)
	return xid, ok
}

// GlobalAT returns an [aspect.Interceptor] that provides the AT
// @GlobalTransactional equivalent: it begins a global transaction, runs the
// business method with the XID injected into the context, and then resolves it —
// committing when the method returns no error, rolling back when it does. The
// original business error is always propagated to the caller unchanged; a commit
// or rollback failure is logged by the coordinator's [Observer] / returned to the
// interceptor but does not replace the business outcome.
//
// Unlike Saga's GlobalTransactional and TCC's GlobalTCC, AT needs no per-method
// registry: there are no hand-written steps to look up. Every branch that the
// business method touches registers itself automatically through the
// resource-side interceptor keyed on the XID this aspect injects. Combine with
// [aspect.Only] to scope it to specific methods within a shared chain.
func GlobalAT(coord Coordinator) aspect.Interceptor {
	return aspect.InterceptorFunc(func(jp *aspect.Joinpoint) (any, error) {
		// A nested GlobalAT reuses the outer transaction: if an XID is already on the
		// context, proceed transparently so the inner branches join the outer XID.
		if _, ok := XIDFromContext(jp.Context); ok {
			return jp.Proceed(jp.Context)
		}

		ctx, xid := coord.Begin(jp.Context)
		res, err := jp.Proceed(ctx)
		if err != nil {
			// Roll back on business failure; the business error is what the caller sees.
			_ = coord.Rollback(ctx, xid)
			return res, err
		}
		// Commit on success. A commit (undo-log cleanup) failure does not change the
		// committed business outcome, so it is not surfaced as the call's error.
		_ = coord.Commit(ctx, xid)
		return res, nil
	})
}
