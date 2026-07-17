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

	"go-spring.org/goframe/grpc/idl/echo"
	"go-spring.org/spring/gs"
	"google.golang.org/grpc"
)

func init() {
	// Provide a ServiceRegister bean that binds the EchoServiceImpl onto the raw
	// *grpc.Server via the generated echo.RegisterEchoServiceServer. The gRPC
	// server adapter (see server.go) depends only on this function type, so the
	// concrete service is wired here without the server ever knowing about the
	// generated echo service — replacing the grpcx scaffold's inline
	// echo.RegisterEchoServiceServer(s.Server, &EchoServiceImpl{}) block.
	gs.Provide(func() ServiceRegister {
		return func(s grpc.ServiceRegistrar) {
			echo.RegisterEchoServiceServer(s, &EchoServiceImpl{})
		}
	})
}

// EchoServiceImpl implements the EchoService gRPC interface defined in
// idl/echo.proto. UnimplementedEchoServiceServer is embedded per gRPC
// forward-compat guidance so new RPCs added to the IDL do not break the build
// on this side until they are implemented explicitly.
type EchoServiceImpl struct {
	echo.UnimplementedEchoServiceServer
}

// Echo returns the request message unchanged, giving the client a
// deterministic value to assert on.
func (s *EchoServiceImpl) Echo(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
	return &echo.EchoResponse{Message: req.Message}, nil
}
