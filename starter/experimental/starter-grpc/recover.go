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

// recover.go is the gRPC seam of the unified panic policy: a handler panic is
// reported through the shared goutil chain (structured log via
// go-spring.org/log) and converted into codes.Internal, instead of crashing
// the whole process — grpc-go recovers nothing by itself.
//
// The interceptor sits innermost (right before the handler), so the converted
// error unwinds back through tracing/metrics/resilience and is fully
// observed — the same placement rationale FaultUnaryInterceptor documents for
// injected errors.

package StarterGrpc

import (
	"context"

	"go-spring.org/stdlib/goutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoverUnaryInterceptor recovers a unary handler panic and converts it into
// codes.Internal. Install it innermost so outer interceptors observe the
// failure on their normal error path.
func RecoverUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				goutil.ReportPanic(ctx, r)
				err = status.Errorf(codes.Internal, "panic in %s: %v", info.FullMethod, r)
			}
		}()
		return handler(ctx, req)
	}
}

// RecoverStreamInterceptor is the streaming twin of
// RecoverUnaryInterceptor: it recovers a panic raised while the handler sets
// up or services the stream and converts it into codes.Internal.
func RecoverStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				goutil.ReportPanic(ss.Context(), r)
				err = status.Errorf(codes.Internal, "panic in %s: %v", info.FullMethod, r)
			}
		}()
		return handler(srv, ss)
	}
}
