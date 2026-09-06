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

	"go-spring.org/cloud/governance/traffic/canonical"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// LoadTestUnaryInterceptor tags the handler context as load-test traffic when
// the incoming gRPC metadata carries the marker key (default x-loadtest). It is
// the gRPC inbound companion to cloud/governance/traffic's outbound carrier injection,
// letting a load-test flag ride a gRPC hop end to end. Installed first in the
// server interceptor chain (outermost), so tracing, metrics, resilience and the
// handler all see the marker via traffic.IsLoadTest(ctx).
//
// Without the marker the interceptor is a no-op. gRPC metadata keys are
// lower-case, so the key is canonical.MetaKeyLoadTest ("x-loadtest") rather than
// the HTTP header spelling.
func LoadTestUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = extractLoadTest(ctx)
		return handler(ctx, req)
	}
}

// LoadTestStreamInterceptor is the streaming-RPC counterpart of
// LoadTestUnaryInterceptor.
func LoadTestStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := extractLoadTest(ss.Context())
		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: ctx})
	}
}

// extractLoadTest tags ctx from the incoming metadata when the marker is
// present. Missing metadata or marker => ctx unchanged.
func extractLoadTest(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return canonical.ExtractCarrier(ctx, canonical.Carrier(md), canonical.MetaKeyLoadTest, "grpc-metadata")
}
