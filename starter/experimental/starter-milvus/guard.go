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

// guard.go is the "command seam" concept of this starter: the per-RPC
// resilience guard, wired as gRPC client interceptors on the SDK's dial
// options. It mirrors starter-tdengine's guardedConn and starter-elasticsearch's
// resilience round-tripper — the guard rides the transport the SDK actually
// uses, so every Milvus RPC (collection, index, search, insert, ...) is
// protected without any opt-in at the call site.
package StarterMilvus

import (
	"context"
	"sync"

	"go-spring.org/cloud/governance/resilience"
	"google.golang.org/grpc"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

// guardSlot carries the executor + resource Init arms after gs field-injects
// Observability. The interceptors consult it on every RPC; before Init it is
// transparent (nil exec) — the same shape as starter-tdengine's clientSlot.
type guardSlot struct {
	mu       sync.RWMutex
	exec     resilience.Executor
	resource string
}

// arm installs the executor + resource the interceptors route through.
func (s *guardSlot) arm(exec resilience.Executor, resource string) {
	s.mu.Lock()
	s.exec = exec
	s.resource = resource
	s.mu.Unlock()
}

// load returns the armed executor + resource, or (nil, "") before Init.
func (s *guardSlot) load() (resilience.Executor, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exec, s.resource
}

// guardDialOptions returns the gRPC dial options that route every RPC through
// the slot's executor once armed. The SDK skips its own DefaultGrpcOpts when
// custom DialOptions are supplied, so they are re-added first (keepalive,
// connect backoff, 2GB recv limit) — the starter's options are additive, not a
// replacement.
func guardDialOptions(slot *guardSlot) []grpc.DialOption {
	opts := append([]grpc.DialOption{}, client.DefaultGrpcOpts...)
	opts = append(opts,
		grpc.WithChainUnaryInterceptor(unaryGuard(slot)),
		grpc.WithChainStreamInterceptor(streamGuard(slot)),
	)
	return opts
}

// unaryGuard returns the unary interceptor: each call runs inside
// exec.Execute, so a rate limiter rejects or an open circuit short-circuits
// the RPC before it is attempted.
func unaryGuard(slot *guardSlot) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		if exec, resource := slot.load(); exec != nil {
			return exec.Execute(ctx, resource, func(ctx context.Context) error {
				return invoker(ctx, method, req, reply, cc, opts...)
			})
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// streamGuard returns the stream interceptor: the stream's open (which sends
// the initial request) runs inside exec.Execute. Per-message reads afterwards
// are driven by the caller and not additionally guarded, matching how the
// other client starters guard the request, not the response body.
func streamGuard(slot *guardSlot) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		if exec, resource := slot.load(); exec != nil {
			var cs grpc.ClientStream
			err := exec.Execute(ctx, resource, func(ctx context.Context) error {
				var e error
				cs, e = streamer(ctx, desc, cc, method, opts...)
				return e
			})
			return cs, err
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
