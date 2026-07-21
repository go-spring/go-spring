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
	goframegrpc "go-spring.org/starter-goframe/grpc"
	"google.golang.org/grpc"
)

func init() {
	// Provide the starter's ServiceRegister bean that binds the EchoServiceImpl
	// onto the raw *grpc.Server via the generated echo.RegisterEchoServiceServer.
	// Importing the starter package (goframegrpc) triggers its module init, which
	// registers the grpcx server as a gs.Server; this bean is the only wiring the
	// application supplies — the server lifecycle and log bridge live in the
	// starter now (they used to be the deleted provider/server.go).
	gs.Provide(func() goframegrpc.ServiceRegister {
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
