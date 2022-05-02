/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package main implements a server for Greeter service.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-spring/spring-core/grpc"
	"github.com/go-spring/spring-core/gs"
	pb "github.com/go-spring/starter-grpc/example/helloworld"

	_ "github.com/go-spring/starter-grpc/server"
)

func init() {
	gs.Object(new(GreeterServer)).Init(func(s *GreeterServer) {
		gs.GrpcServer("helloworld.Greeter", &grpc.Server{
			Register: pb.RegisterGreeterServer,
			Service:  s,
		})
	})
}

type GreeterServer struct {
	AppName string `value:"${spring.application.name}"`
}

func (s *GreeterServer) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName() + " from " + s.AppName}, nil
}

func main() {
	gs.Property("spring.application.name", "GreeterServer")
	gs.Property("grpc.server.port", 50051)
	fmt.Println("application exit: ", gs.Web(false).Run())
}
