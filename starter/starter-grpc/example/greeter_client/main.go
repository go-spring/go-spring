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

// Package main implements a client for Greeter service.
package main

import (
	"context"
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	pb "github.com/go-spring/starter-grpc/example/helloworld"

	_ "github.com/go-spring/starter-grpc/client"
)

const (
	defaultName = "world"
)

func init() {
	gs.GrpcClient(pb.NewGreeterClient, "greeter")
	gs.Object(&runner{}).Export((*gs.AppRunner)(nil))
}

type runner struct {
	GreeterClient pb.GreeterClient `autowire:""`
}

func (r *runner) Run(ctx gs.Context) {
	resp, err := r.GreeterClient.SayHello(context.TODO(), &pb.HelloRequest{Name: defaultName})
	web.ERROR.Panic(err).When(err != nil)
	fmt.Println("Greeting: " + resp.GetMessage())
	go gs.ShutDown()
}

func main() {
	gs.Property("grpc.endpoint.greeter.address", "127.0.0.1:50051")
	gs.Property("spring.application.name", "GreeterClient")
	fmt.Println("application exit: ", gs.Web(false).Run())
}
