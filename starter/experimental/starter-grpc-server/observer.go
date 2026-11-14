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

// observer.go is grpc-server's observability seam: the interceptors that emit
// this starter's request-plane signals, and the default OTel-backed adapter.

package StarterGrpcServer

import (
	"go-spring.org/spring/gs"
	"google.golang.org/grpc"
)

// Observer supplies grpc-server's METRIC interceptors. It is the seam: the
// default is OTel, and an application may contribute its own adapter. Tracing
// is the STANDARD OTel capability (see [TracingUnaryInterceptor] /
// [TracingStreamInterceptor]) and is not part of this contract.
type Observer interface {
	// MetricsUnary returns the unary interceptor that records request metrics.
	MetricsUnary() (grpc.UnaryServerInterceptor, error)
	// MetricsStream returns the stream interceptor that records stream metrics.
	MetricsStream() (grpc.StreamServerInterceptor, error)
}

// NewOTelObserver returns the default OTel-backed [Observer].
func NewOTelObserver() (Observer, error) { return otelObserver{}, nil }

// otelObserver is the default [Observer]; its interceptor bodies live in
// metrics.go and tracing.go.
type otelObserver struct{}

// TracingUnaryInterceptor returns the default unary tracing interceptor.
func TracingUnaryInterceptor() grpc.UnaryServerInterceptor { return tracingUnary() }

// TracingStreamInterceptor returns the default stream tracing interceptor.
func TracingStreamInterceptor() grpc.StreamServerInterceptor { return tracingStream() }

// MetricsUnaryInterceptor returns the default unary metrics interceptor.
func MetricsUnaryInterceptor() (grpc.UnaryServerInterceptor, error) {
	return otelObserver{}.MetricsUnary()
}

// MetricsStreamInterceptor returns the default stream metrics interceptor.
func MetricsStreamInterceptor() (grpc.StreamServerInterceptor, error) {
	return otelObserver{}.MetricsStream()
}

// The package's default [Observer] bean: an application that contributes its
// own observer takes precedence.
func init() {
	gs.Provide(NewOTelObserver).Condition(gs.OnMissingBean[Observer]()).Caller(1)
}
