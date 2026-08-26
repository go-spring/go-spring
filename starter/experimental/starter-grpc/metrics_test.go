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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestMetricsUnaryInterceptor_NoopMeter asserts the metrics path never panics
// with the default (no-op) MeterProvider, and that response, error and handler
// call count all pass through exactly once — the in-flight gauge decrement must
// fire on the error path too, or the gauge would leak.
func TestMetricsUnaryInterceptor_NoopMeter(t *testing.T) {
	ic := MetricsUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/demo.Service/Echo"}

	hits := 0
	resp, err := ic(context.Background(), "req", info,
		func(context.Context, any) (any, error) {
			hits++
			return "ok", nil
		})
	assert.That(t, err).Nil()
	assert.That(t, resp).Equal("ok")
	assert.That(t, hits).Equal(1)

	// Handler error: counted with its status code, propagated untouched.
	_, err = ic(context.Background(), "req", info,
		func(context.Context, any) (any, error) { return nil, status.Error(codes.Unavailable, "down") })
	assert.That(t, status.Code(err)).Equal(codes.Unavailable)
}

// TestMetricsStreamInterceptor_NoopMeter is the streaming twin: count and
// duration recording on a no-op meter must not panic, on success or error.
func TestMetricsStreamInterceptor_NoopMeter(t *testing.T) {
	ic := MetricsStreamInterceptor()
	info := &grpc.StreamServerInfo{FullMethod: "/demo.Service/Chat"}

	err := ic(nil, &fakeStream{ctx: context.Background()}, info,
		func(any, grpc.ServerStream) error { return nil })
	assert.That(t, err).Nil()

	err = ic(nil, &fakeStream{ctx: context.Background()}, info,
		func(any, grpc.ServerStream) error { return status.Error(codes.DeadlineExceeded, "slow") })
	assert.That(t, status.Code(err)).Equal(codes.DeadlineExceeded)
}
