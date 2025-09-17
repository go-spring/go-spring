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
	"fmt"
	"log"
	"os"
	"time"

	"thriftsvr/idl/proto"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Object(&Controller{})
	gs.Provide(func(c *Controller) thrift.TProcessor {
		return proto.NewEchoServiceProcessor(c)
	})
}

type Controller struct{}

func (c *Controller) Echo(ctx context.Context, req *proto.EchoRequest) (r *proto.EchoResponse, err error) {
	return &proto.EchoResponse{Message: req.Message}, nil
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")
	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()
	gs.Run()
}

func runTest() {
	transport := thrift.NewTSocketConf(":9292", nil)
	defer transport.Close()

	protocolFactory := thrift.NewTBinaryProtocolFactoryConf(nil)
	client := proto.NewEchoServiceClientFactory(transport, protocolFactory)

	if err := transport.Open(); err != nil {
		log.Fatalf("Error opening transport: %v", err)
	}

	response, err := client.Echo(context.Background(), &proto.EchoRequest{Message: "Hello, Thrift!"})
	if err != nil {
		log.Fatalf("Error calling Echo: %v", err)
	}

	fmt.Println("Response from server:", response.Message)

	gs.ShutDown()
}
