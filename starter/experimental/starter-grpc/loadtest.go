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

package StarterGrpc

import (
	"context"

	"go-spring.org/cloud/propagate"
	"go-spring.org/cloud/traffic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// LoadTestUnaryInterceptor tags the handler context as load-test traffic when
// the incoming gRPC metadata carries prop's marker key. It is the gRPC inbound
// companion to cloud/traffic's outbound carrier injection, letting a
// load-test flag ride a gRPC hop end to end. Installed first in the server
// interceptor chain (outermost), so tracing, metrics, resilience and the handler
// all see the marker via prop.IsLoadTest(ctx).
//
// Without the marker the interceptor is a no-op. gRPC metadata keys must be
// lower-case, and go-spring's metadata key is spelled that way ("x-loadtest")
// rather than like its HTTP header. A nil propagator means go-spring's default
// convention.
func LoadTestUnaryInterceptor(prop traffic.Propagator) grpc.UnaryServerInterceptor {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = extractLoadTest(prop, ctx)
		return handler(ctx, req)
	}
}

// LoadTestStreamInterceptor is the streaming-RPC counterpart of
// LoadTestUnaryInterceptor.
func LoadTestStreamInterceptor(prop traffic.Propagator) grpc.StreamServerInterceptor {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := extractLoadTest(prop, ss.Context())
		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: ctx})
	}
}

// extractLoadTest tags ctx from the incoming metadata when prop's marker is
// present. Missing metadata or marker => ctx unchanged.
func extractLoadTest(prop traffic.Propagator, ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return prop.Extract(ctx, propagate.MultiMap(md))
}
