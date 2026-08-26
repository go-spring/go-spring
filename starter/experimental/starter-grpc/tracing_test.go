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
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestTracingUnaryInterceptor_NoopGlobals proves the "on by default, no-op
// without starter-otel" contract: with the default (no-op) OTel globals the
// interceptor starts and ends a span on both the success and error paths
// without panicking, hands the handler a context carrying a span, and passes
// the response/error through untouched.
func TestTracingUnaryInterceptor_NoopGlobals(t *testing.T) {
	ic := TracingUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/demo.Service/Echo"}

	var span trace.Span
	resp, err := ic(context.Background(), "req", info,
		func(ctx context.Context, _ any) (any, error) {
			span = trace.SpanFromContext(ctx)
			return "ok", nil
		})
	assert.That(t, err).Nil()
	assert.That(t, resp).Equal("ok")
	assert.That(t, span != nil).True() // no-op span, but a span is on the ctx

	// Error path: status extraction + SetStatus/RecordError on the no-op span.
	_, err = ic(context.Background(), "req", info,
		func(context.Context, any) (any, error) { return nil, status.Error(codes.NotFound, "missing") })
	assert.That(t, status.Code(err)).Equal(codes.NotFound)
}

// TestTracingUnaryInterceptor_ExtractsIncomingMetadata covers the propagation
// half: incoming gRPC metadata is run through the global propagator (a no-op
// extract by default) without error even when metadata is present.
func TestTracingUnaryInterceptor_ExtractsIncomingMetadata(t *testing.T) {
	ic := TracingUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/demo.Service/Echo"}
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"))
	resp, err := ic(ctx, nil, info, func(context.Context, any) (any, error) { return "ok", nil })
	assert.That(t, err).Nil()
	assert.That(t, resp).Equal("ok")
}

// TestTracingStreamInterceptor_NoopGlobals is the streaming twin: the whole
// stream lifecycle is wrapped in a span, the handler sees a stream whose
// Context carries that span, and the handler error passes through.
func TestTracingStreamInterceptor_NoopGlobals(t *testing.T) {
	ic := TracingStreamInterceptor()
	info := &grpc.StreamServerInfo{FullMethod: "/demo.Service/Chat"}

	var span trace.Span
	err := ic(nil, &fakeStream{ctx: context.Background()}, info,
		func(_ any, ss grpc.ServerStream) error {
			span = trace.SpanFromContext(ss.Context())
			return status.Error(codes.Internal, "boom")
		})
	assert.That(t, status.Code(err)).Equal(codes.Internal)
	assert.That(t, span != nil).True()
}

// TestServiceName pins the rpc.service attribute extraction from a full
// method path, including the degenerate shapes.
func TestServiceName(t *testing.T) {
	assert.That(t, serviceName("/pkg.Service/Method")).Equal("pkg.Service")
	assert.That(t, serviceName("/NoSub/")).Equal("NoSub")
	assert.That(t, serviceName("/OnlyPath")).Equal("OnlyPath")
	assert.That(t, serviceName("bare")).Equal("bare")
	assert.That(t, serviceName("")).Equal("")
}
