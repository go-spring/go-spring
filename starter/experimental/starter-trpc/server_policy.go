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

package StarterTrpc

import (
	"context"
	"errors"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/guard"
	"go-spring.org/cloud/observability"
	"trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/errs"
	"trpc.group/trpc-go/trpc-go/filter"
)

// GuardServerFilter is the inbound governance filter: one admission judgment
// (rate limit / bulkhead / breaker / fault) before the handler, and one
// completion after it. It is registered by the starter as "governance" and
// included in the server's default filter chain.
func GuardServerFilter(in *governance.Inbound) filter.ServerFilter {
	return func(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
		op := observability.ServerOperation{Action: actionName(ctx)}
		var resp interface{}
		err := in.Run(ctx, op, func(ctx context.Context) error {
			var e error
			resp, e = next(ctx, req)
			return e
		})
		if err != nil {
			var ref *governance.Refusal
			if errors.As(err, &ref) {
				return nil, refusalError(ref)
			}
			return nil, err
		}
		return resp, nil
	}
}

// actionName names the inbound RPC: the entrance ("trpc") plus the callee
// method tRPC resolved for this request.
func actionName(ctx context.Context) string {
	return trpc.Message(ctx).CalleeMethod()
}

// refusalError maps a guard refusal onto tRPC's return codes, so callers that
// branch on code can tell throttling from an arbitrary handler failure.
func refusalError(ref *governance.Refusal) error {
	switch ref.Verdict() {
	case guard.VerdictRateLimited, guard.VerdictBulkheadFull:
		return errs.New(errs.RetServerThrottled, ref.Error())
	case guard.VerdictCircuitOpen, guard.VerdictDegraded:
		return errs.New(errs.RetServerOverload, ref.Error())
	case guard.VerdictFaultInjected:
		return errs.New(errs.RetServerSystemErr, ref.Error())
	default:
		return ref
	}
}
