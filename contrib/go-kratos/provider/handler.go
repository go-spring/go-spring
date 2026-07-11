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
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "go-spring.org/go-kratos/api/helloworld/v1"
	"go-spring.org/go-kratos/internal/service"
	"go-spring.org/spring/gs"
)

func init() {
	// Provide a ServiceRegister bean that binds the GreeterService to both the
	// kratos HTTP and gRPC transport servers. The KratosServer adapter (see
	// server.go) depends only on this function type, so the concrete service is
	// wired here without the adapter ever knowing about v1.GreeterServer.
	gs.Provide(func(greeter *service.GreeterService) ServiceRegister {
		return func(hs *khttp.Server, gs *kgrpc.Server) error {
			v1.RegisterGreeterHTTPServer(hs, greeter)
			v1.RegisterGreeterServer(gs, greeter)
			return nil
		}
	})
}
