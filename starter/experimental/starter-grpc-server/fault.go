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

	"go-spring.org/cloud/fault"
	"google.golang.org/grpc"
)

// FaultUnaryInterceptor injects faults into inbound unary RPCs: per the
// injector's rules a request is made to fail (injected error) or slow down
// (configured latency) before the handler runs. It is the server-side
// counterpart to the client starters' fault.ClientManager.WrapClientExecutor — letting an operator
// "set fire" to a running gRPC server to verify its observe and upstream
// clients' retry/breaker behavior. Installed innermost (after tracing/metrics/
// resilience) so the injected error flows back through those layers and is
// observed.
//
// inj is the governance starter's injector bean, held for the life of the
// interceptor: the center hot-swaps its config in place
// ([fault.ServerManager.Refresh]) rather than replacing the bean, so the captured
// reference always sees the live config. The bean is always present (an absent
// bean fails startup) and inert by default, so this is always installed.
func FaultUnaryInterceptor(inj *fault.ServerManager) grpc.UnaryServerInterceptor {
	srv := inj.NewServer()
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, err := srv.Fire(ctx, faultTarget(info.FullMethod)); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// FaultStreamInterceptor is the streaming-RPC counterpart of
// FaultUnaryInterceptor. Latency is applied before the stream handler runs; an
// injected error aborts it. inj is the governance starter's injector bean, as in
// FaultUnaryInterceptor.
func FaultStreamInterceptor(inj *fault.ServerManager) grpc.StreamServerInterceptor {
	fs := inj.NewServer()
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if _, err := fs.Fire(ss.Context(), faultTarget(info.FullMethod)); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

// faultTarget names the inbound RPC for the fault rules: the entrance ("grpc",
// one label for everything this process serves) plus the interface being
// invoked — the full method, which is gRPC's own name for it and the finest
// addressable unit a rule can target. The method is the ACTION, not the service:
// keeping it out of Service is what lets one rule cover a whole service and
// another a single method of it.
func faultTarget(fullMethod string) fault.Target {
	return fault.Target{Service: "grpc", Action: fullMethod}
}
