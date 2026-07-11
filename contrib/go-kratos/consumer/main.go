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
	"time"

	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	transgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	clientv3 "go.etcd.io/etcd/client/v3"
	v1 "go-spring.org/go-kratos/api/helloworld/v1"
)

// The consumer never learns the provider's host:port. It builds an etcd-backed
// registry.Discovery and asks kratos to dial the endpoint "discovery:///<name>"
// — the same service name the provider registered under. kratos resolves a
// live provider instance from etcd, dials it via gRPC, and we assert on the
// echo.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	svcName := flag.String("service", "kratos-greeter", "kratos service name to resolve")
	flag.Parse()

	ctx := context.Background()

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{*registryAddr},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd client: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	r := etcd.New(cli)

	conn, err := transgrpc.DialInsecure(ctx,
		transgrpc.WithEndpoint("discovery:///"+*svcName),
		transgrpc.WithDiscovery(r),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial discovered provider: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	resp, err := v1.NewGreeterClient(conn).SayHello(ctx, &v1.HelloRequest{Name: "Kratos"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling SayHello: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider:", resp.Message)
	if resp.Message != "Hello Kratos" {
		fmt.Fprintf(os.Stderr, "unexpected reply: %q\n", resp.Message)
		os.Exit(1)
	}
}
