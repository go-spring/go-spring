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
	"fmt"

	pb "github.com/go-spring/examples/spring-boot-grpc/helloworld"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-gin"
	_ "github.com/go-spring/starter-grpc/client"
)

const (
	defaultName = "world"
)

func init() {
	gs.Provide(new(GreeterClientController)).Init(func(c *GreeterClientController) {
		gs.GetMapping("/", c.index)
	})
}

type GreeterClientController struct {
	GreeterClient pb.GreeterClient `autowire:""`
}

func (c *GreeterClientController) index(ctx web.Context) {
	r, err := c.GreeterClient.SayHello(ctx.Request().Context(), &pb.HelloRequest{Name: defaultName})
	web.ERROR.Panic(err).When(err != nil)
	ctx.String("Greeting: " + r.GetMessage())
}

func init() {
	gs.GrpcClient(pb.NewGreeterClient, "greeter-client")
}

func main() {
	gs.Property("grpc.endpoint.greeter-client.address", "127.0.0.1:50051")
	gs.Property("spring.application.name", "GreeterClient")
	fmt.Println("application exit: ", gs.Run())
}
