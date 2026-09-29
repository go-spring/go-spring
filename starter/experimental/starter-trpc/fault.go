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

	"go-spring.org/cloud/governance/fault"
	"trpc.group/trpc-go/trpc-go/filter"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is govern.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// FaultServerFilter is a tRPC ServerFilter that injects faults (latency/error)
// into inbound RPCs per the injector's rules. The starter registers it under
// the name "fault"; add "fault" to a service's filter chain to activate it. It
// is the tRPC server-side counterpart to the client starters'
// fault.WrapClientExecutor, letting an operator "set fire" to a running server.
//
// inj is the governance starter's injector bean, held for the life of the
// filter: the center hot-swaps its config in place
// ([fault.Injector.SetConfig]) rather than replacing the bean, so the captured
// reference always sees the live config. A nil inj makes fault.ApplyServer a
// transparent pass-through.
func FaultServerFilter(inj *fault.Injector) filter.ServerFilter {
	return func(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
		var resp interface{}
		err := fault.ApplyServer(ctx, inj, "trpc", func() error {
			var e error
			resp, e = next(ctx, req)
			return e
		})
		return resp, err
	}
}
