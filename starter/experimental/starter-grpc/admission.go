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
	"errors"

	"go-spring.org/cloud/governance/resilience"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// resilienceInterceptors are the unary/stream wrappers built from the server's
// resilience policy, if any.
type resilienceInterceptors struct {
	unary grpc.UnaryServerInterceptor
}

// buildResilienceInterceptors constructs an inbound-admission executor from the
// injected [resilience.Manager] and returns the unary interceptor that runs each
// RPC through it. The manager is the governance starter's bean, so this server
// gets its rate-limit / bulkhead / breaker limits from the governance document's
// SERVER block (spring.governance.server.*)
// WITHOUT naming *governance.Center. A nil manager — a standalone call, or an app
// that does not import starter-governance — is normalized to an unarmed one,
// whose executor is a transparent pass-through (fn runs once, untouched). The
// executor handle resolves its backing implementation per call and follows the
// manager's hot-reload. The executor is wrapped with observe-resilience so
// breaker trips / rate rejects / bulkhead rejects emit span + counter +
// histogram.
//
// Inbound resilience is admission control: set RateLimit / MaxConcurrent to
// protect the server from overload; ErrRateLimited / ErrBulkheadFull /
// ErrCircuitOpen surface as the executor's error on the RPC. Do NOT configure
// retry for inbound (a handler that has already produced side effects cannot be
// replayed) — leave MaxRetries at 0: inbound serving is not idempotent.
func (s *SimpleGrpcServer) buildResilienceInterceptors() (resilienceInterceptors, bool) {
	mgr := s.mgr
	if mgr == nil {
		mgr = resilience.NewManager()
	}
	service := resilience.ServiceLabel("grpc", s.cfg.Addr)
	exec := mgr.ServerExecutorFor("grpc", service)
	return resilienceInterceptors{
		unary: resilienceUnaryInterceptor(exec, service),
	}, true
}

// resilienceUnaryInterceptor runs each unary RPC through the executor so the
// configured rate-limit / bulkhead / breaker (admission) policy is enforced
// before the handler runs.
//
// ServerPolicy rejections are mapped to semantic gRPC status codes so consumers
// that branch on status code (retry policies, circuit breakers, dashboards)
// can tell "this client is being throttled" from an arbitrary handler failure:
// rate/bulkhead rejections are ResourceExhausted, an open circuit is
// Unavailable. Without the mapping they cross the wire as codes.Unknown,
// which no consumer can act on.
func resilienceUnaryInterceptor(exec resilience.ServerExecutor, service string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var resp any
		err := exec.Execute(ctx, func(ctx context.Context) error {
			var e error
			resp, e = handler(ctx, req)
			return e
		})
		return resp, mapServerPolicyError(err)
	}
}

// mapServerPolicyError translates resilience admission rejections into semantic
// gRPC status errors; every other error (handler failure, injected fault, ...)
// passes through untouched.
func mapServerPolicyError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, resilience.ErrRateLimited), errors.Is(err, resilience.ErrBulkheadFull):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, resilience.ErrCircuitOpen):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return err
	}
}
