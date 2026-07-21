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

	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	v1 "go-spring.org/go-kratos-grpc/idl/helloworld/v1"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	kratosgrpc "go-spring.org/starter-kratos/grpc"
)

func init() {
	// Provide the single-argument ServiceRegister bean starter-kratos/grpc
	// depends on: it binds GreeterService onto the kratos gRPC transport server.
	// The starter's GrpcServer only materializes because this bean exists, and it
	// never learns about v1.GreeterServer — registration is entirely here.
	gs.Provide(func() kratosgrpc.ServiceRegister {
		return func(gs *kgrpc.Server) error {
			v1.RegisterGreeterServer(gs, &GreeterService{})
			return nil
		}
	})
}

// GreeterService implements the GreeterServer interface generated from
// helloworld.proto. The greeting is folded directly into the handler rather
// than the kratos scaffold's internal/{biz,service,data} layering, matching the
// flat provider/handler.go shape the other contrib examples use.
type GreeterService struct {
	v1.UnimplementedGreeterServer
}

// SayHello echoes the request name back as "Hello <name>", giving the consumer
// a deterministic value to assert on over gRPC.
func (s *GreeterService) SayHello(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	// Business log line, emitted through go-spring's log module (configured as a
	// JSON FileLogger in provider/conf/app.properties). kratos' own framework
	// logs are bridged into the same pipeline by starter-kratos.
	log.Infof(ctx, log.TagBizDef, "SayHello name=%s", in.Name)
	return &v1.HelloReply{Message: "Hello " + in.Name}, nil
}
