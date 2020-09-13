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
	"log"

	pb "github.com/go-spring/examples/spring-boot-grpc/helloworld"
	"github.com/go-spring/spring-boot"
	_ "github.com/go-spring/starter-grpc-server"
)

func init() {
	SpringBoot.RegisterGRpcServer(pb.RegisterGreeterServer,
		"helloworld.Greeter",
		new(GreeterServer),
	).ConditionOnOptionalPropertyValue("greeter-server.enable", true)
}

type GreeterServer struct {
	AppName string `value:"${spring.application.name}"`
}

func (s *GreeterServer) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName() + " from " + s.AppName}, nil
}

func main() {
	SpringBoot.SetProperty("spring.application.name", "GreeterServer")
	SpringBoot.SetProperty("grpc.server.port", 50051)
	SpringBoot.RunApplication()
}
