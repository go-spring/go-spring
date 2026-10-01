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

	"go-spring.org/cloud/propagate"
	"go-spring.org/cloud/traffic"
	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/filter"
)

// LoadTestServerFilter is a tRPC ServerFilter that tags the handler context as
// load-test traffic when the inbound server metadata carries prop's marker key.
// The starter registers it under the name "loadtest" (filter.Register); add
// "loadtest" to the service filter chain — ideally first, so the marker is on the
// context before tracing, metrics and the handler run, letting every downstream
// layer branch on prop.IsLoadTest(ctx).
//
// It is the tRPC inbound companion to cloud/traffic's outbound carrier
// injection, letting a load-test flag ride a tRPC hop end to end. Without the
// marker the filter is a no-op pass-through. A nil propagator means go-spring's
// default convention.
func LoadTestServerFilter(prop traffic.Propagator) filter.ServerFilter {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return func(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
		// tRPC metadata values are []byte; walk them into the seam's string
		// multi-map.
		md := trpc.Message(ctx).ServerMetaData()
		h := propagate.MultiMap{}
		for k, v := range md {
			h[k] = []string{string(v)}
		}
		ctx = prop.Extract(ctx, h)
		return next(ctx, req)
	}
}
