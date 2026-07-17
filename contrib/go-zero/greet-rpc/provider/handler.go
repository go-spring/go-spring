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

package main

import (
	"context"

	"go-spring.org/spring/gs"
	gozerozrpc "go-spring.org/starter-go-zero/zrpc"
	"google.golang.org/grpc"

	greet "greetrpc/idl"
)

func init() {
	// Provide a ServiceRegister bean that binds the GreetProvider to the
	// underlying grpc.Server. starter-go-zero/zrpc owns the ZrpcServer adapter
	// (imported here for its gozerozrpc.ServiceRegister type, which also
	// triggers the starter's init); its server depends only on this function
	// type, so the concrete greet.GreetServer is wired here without the server
	// ever knowing about it.
	gs.Provide(func() gozerozrpc.ServiceRegister {
		return func(grpcServer *grpc.Server) {
			greet.RegisterGreetServer(grpcServer, &GreetProvider{})
		}
	})
}

// GreetProvider implements the greet.GreetServer interface generated from
// greet.proto.
type GreetProvider struct {
	greet.UnimplementedGreetServer
}

// Greet echoes the request name back as the greeting, giving the consumer a
// deterministic value to assert on.
func (s *GreetProvider) Greet(ctx context.Context, req *greet.GreetReq) (*greet.GreetResp, error) {
	return &greet.GreetResp{Greeting: req.Name}, nil
}
