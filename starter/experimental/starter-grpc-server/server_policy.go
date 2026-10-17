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

package StarterGrpcServer

import (
	"context"
	"errors"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/guard"
	"go-spring.org/cloud/label"
	"go-spring.org/cloud/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// resilienceInterceptors are the unary/stream wrappers built from the server's
// resilience policy, if any.
type resilienceInterceptors struct {
	unary  grpc.UnaryServerInterceptor
	stream grpc.StreamServerInterceptor
}

// buildResilienceInterceptors constructs an inbound-inbound executor from the
// injected [guard.Manager] and returns the unary interceptor that runs each
// RPC through it. The manager is the governance starter's bean, so this server
// gets its rate-limit / bulkhead / breaker limits from the governance document's
// SERVER block (spring.governance.server.*)
// WITHOUT naming *governance.Center. A nil manager — a standalone call, or an app
// that does not import — is normalized to an unarmed one,
// whose executor is a transparent pass-through (fn runs once, untouched). The
// executor handle resolves its backing implementation per call and follows the
// manager's hot-reload. The executor is wrapped with observe-resilience so
// breaker trips / rate rejects / bulkhead rejects emit span + counter +
// histogram.
//
// Inbound resilience is inbound control: set RateLimit / MaxConcurrent to
// protect the server from overload; ErrRateLimited / ErrBulkheadFull /
// ErrCircuitOpen surface as the executor's error on the RPC. Do NOT configure
// retry for inbound (a handler that has already produced side effects cannot be
// replayed) — leave MaxRetries at 0: inbound serving is not idempotent.
func (s *SimpleGrpcServer) buildResilienceInterceptors() (resilienceInterceptors, bool) {
	service := label.ServiceLabel("grpc", s.cfg.Addr)
	// The center composes the observe object with the guard (and normalizes a nil
	// center to an unarmed authority), so this package names neither observability
	// nor the manager's raw handle.
	in := s.center.InboundFor("grpc", service)
	return resilienceInterceptors{
		unary:  resilienceUnaryInterceptor(in, service),
		stream: resilienceStreamInterceptor(in),
	}, true
}

// resilienceUnaryInterceptor runs each unary RPC through the executor so the
// configured rate-limit / bulkhead / breaker (inbound) policy is enforced
// before the handler runs.
//
// ServerPolicy rejections are mapped to semantic gRPC status codes so consumers
// that branch on status code (retry policies, circuit breakers, dashboards)
// can tell "this client is being throttled" from an arbitrary handler failure:
// rate/bulkhead rejections are ResourceExhausted, an open circuit is
// Unavailable. Without the mapping they cross the wire as codes.Unknown,
// which no consumer can act on.
func resilienceUnaryInterceptor(in *governance.Inbound, service string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var resp any
		op := observability.ServerOperation{Action: info.FullMethod}
		err := in.Run(ctx, op, func(ctx context.Context) error {
			var e error
			resp, e = handler(ctx, req)
			return e
		})
		return resp, mapServerPolicyError(err)
	}
}

// resilienceStreamInterceptor runs a whole stream under the same inbound guard
// as a unary call: admission happens once when the stream opens, the handler
// runs with the derived context, and settlement happens when the handler
// returns. Fault injection therefore lives inside the guard for streams too.
func resilienceStreamInterceptor(in *governance.Inbound) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		op := observability.ServerOperation{Action: info.FullMethod}
		return in.Run(ss.Context(), op, func(ctx context.Context) error {
			return handler(srv, &contextServerStream{ServerStream: ss, ctx: ctx})
		})
	}
}

// contextServerStream hands the handler the context derived by the inbound
// guard (span, handling budget).
type contextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context { return s.ctx }

// mapServerPolicyError translates an inbound refusal into a semantic gRPC status
// error; every other error (handler failure, injected fault, an infrastructure
// failure) passes through untouched.
func mapServerPolicyError(err error) error {
	if err == nil {
		return nil
	}
	var ref *governance.Refusal
	if !errors.As(err, &ref) {
		return err
	}
	switch ref.Verdict() {
	case guard.VerdictRateLimited, guard.VerdictBulkheadFull:
		return status.Error(codes.ResourceExhausted, ref.Error())
	case guard.VerdictCircuitOpen, guard.VerdictDegraded:
		return status.Error(codes.Unavailable, ref.Error())
	default:
		return err
	}
}
