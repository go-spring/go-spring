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
	"flag"
	"fmt"
	"os"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	"go-spring.org/goframe/grpc/pbgen/echo"
)

// The consumer never learns the provider's host:port. It registers the same
// etcd registry the provider used, then calls grpcx.Client.MustNewGrpcClientConn
// with the service name: grpcx builds a gsvc://<name> target, goframe's
// discovery resolver watches that name in etcd and returns a live endpoint to
// the underlying *grpc.ClientConn. This is the microservice governance path
// grpcx advertises, not a direct dial.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	svcName := flag.String("service", "goframe.grpc.echo", "service name registered by the provider")
	flag.Parse()

	// Register the etcd registry with grpcx. This must go through
	// grpcx.Resolver.Register (not gsvc.SetRegistry): grpcx's init() already
	// registered a grpc resolver builder bound to the then-nil default
	// registry, so it also has to re-register the builder with a real
	// discovery — which Resolver.Register does and SetRegistry alone does not.
	// Passing the same address the provider registered against is what turns
	// the service name below into a live endpoint.
	grpcx.Resolver.Register(etcdreg.New(*registryAddr))

	conn := grpcx.Client.MustNewGrpcClientConn(*svcName)
	defer conn.Close()

	cli := echo.NewEchoServiceClient(conn)

	want := "Hello, GoFrame gRPC!"
	resp, err := cli.Echo(context.Background(), &echo.EchoRequest{Message: want})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling Echo: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider:", resp.Message)
	if resp.Message != want {
		fmt.Fprintf(os.Stderr, "unexpected echo body: %q\n", resp.Message)
		os.Exit(1)
	}
}
