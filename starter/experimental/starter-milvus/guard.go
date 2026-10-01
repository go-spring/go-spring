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
//
// The interceptors are also this starter's declaration seam: each one puts the
// RPC's semantic identity on the caller's context (see observe.go) before the
// executor runs, so the resilience layer inside the executor — the single
// emitter — names the span, the db.client.* metrics and the access log from it.
// It emits nothing itself.
package StarterMilvus

import (
	"context"
	"sync"

	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	"google.golang.org/grpc"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

// guardSlot carries the executor [NewClient] applies. The interceptors consult
// it on every RPC; before that it is transparent (nil exec) — the same shape as
// starter-tdengine's clientSlot.
type guardSlot struct {
	mu   sync.RWMutex
	exec resilience.ClientExecutor
}

// apply installs the executor the interceptors route through.
func (s *guardSlot) apply(exec resilience.ClientExecutor) {
	s.mu.Lock()
	s.exec = exec
	s.mu.Unlock()
}

// load returns the applied executor, or nil before [NewClient] applies it.
func (s *guardSlot) load() resilience.ClientExecutor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exec
}

// guardDialOptions returns the gRPC dial options that route every RPC through
// the slot's executor once applied. The SDK skips its own DefaultGrpcOpts when
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
//
// The declaration is made on the caller's ctx BEFORE exec.Execute: the emitter
// reads the operation at Execute entry, so a declaration made inside the
// executor's fn — per attempt — would be read by nobody. Placed here, one
// call's signals cover every attempt, retries included.
func unaryGuard(slot *guardSlot) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		if exec := slot.load(); exec != nil {
			ctx = observability.WithOperation(ctx, operation(method))
			return exec.Execute(ctx, func(ctx context.Context) error {
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
//
// The declaration rides the caller's ctx before exec.Execute, exactly as in
// [unaryGuard]: the emitter reads it at Execute entry, so the stream open's
// signals are named like any other RPC's.
func streamGuard(slot *guardSlot) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		if exec := slot.load(); exec != nil {
			ctx = observability.WithOperation(ctx, operation(method))
			var cs grpc.ClientStream
			err := exec.Execute(ctx, func(ctx context.Context) error {
				var e error
				cs, e = streamer(ctx, desc, cc, method, opts...)
				return e
			})
			return cs, err
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
