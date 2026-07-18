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
	greet "go-spring.org/trpc-go/trpc/idl"
	"go-spring.org/trpc-go/trpc/trpcgs"
	trpclog "trpc.group/trpc-go/trpc-go/log"
	"trpc.group/trpc-go/trpc-go/server"
)

func init() {
	// Provide a trpcgs.ServiceRegister bean that binds GreetServiceImpl onto the
	// tRPC server via the generated greet.RegisterGreetServiceService.
	// SimpleTrpcServer depends only on this function type, so the concrete
	// service is wired here without the adapter ever knowing about GreetService.
	gs.Provide(func() trpcgs.ServiceRegister {
		return func(s *server.Server) {
			greet.RegisterGreetServiceService(s, &GreetServiceImpl{})
		}
	})
}

// GreetServiceImpl implements the GreetService defined in greet.proto.
type GreetServiceImpl struct{}

// Greet returns a greeting for the request name. The trpclog.Infof call flows
// through the go-spring log bridge (see trpcgs/logbridge.go), so the framework
// log line is written by go-spring's log pipeline into ../logs/provider.log.
func (s *GreetServiceImpl) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
	trpclog.Infof("greet request: %s", req.Name)
	return &greet.GreetResponse{Greeting: "Hello, " + req.Name + "!"}, nil
}
