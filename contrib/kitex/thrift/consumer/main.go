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

	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
	echo "go-spring.org/kitex/thrift/kitex_gen/echo"
	"go-spring.org/kitex/thrift/kitex_gen/echo/echoservice"
)

// The consumer never learns the provider's host:port. It builds a client bound
// to the same etcd registry and asks for the EchoService by the service name
// the provider registered under; Kitex resolves a live provider address from
// etcd, calls it, and we assert on the echo.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	serviceName := flag.String("service", "echo", "target Kitex service name")
	flag.Parse()

	ctx := context.Background()

	r, err := etcd.NewEtcdResolver([]string{*registryAddr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd resolver: %v\n", err)
		os.Exit(1)
	}

	cli, err := echoservice.NewClient(*serviceName, client.WithResolver(r))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client: %v\n", err)
		os.Exit(1)
	}

	resp, err := cli.Echo(ctx, &echo.EchoRequest{Message: "Hello, Kitex!"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling Echo: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider:", resp.Message)
	if resp.Message != "Hello, Kitex!" {
		fmt.Fprintf(os.Stderr, "unexpected echo body: %q\n", resp.Message)
		os.Exit(1)
	}
}
