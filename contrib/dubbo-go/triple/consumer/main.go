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

	"dubbo.apache.org/dubbo-go/v3/client"
	_ "dubbo.apache.org/dubbo-go/v3/imports"
	"dubbo.apache.org/dubbo-go/v3/registry"
	greet "go-spring.org/dubbo-go/triple/proto"
)

// The consumer never learns the provider's host:port. It builds a client bound
// to the same etcd registry and asks for the GreetService by its interface
// name (greet.GreetService, baked into the generated stub); Dubbo resolves a
// live provider address from etcd, calls it, and we assert on the echo.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	flag.Parse()

	ctx := context.Background()

	cli, err := client.NewClient(
		client.WithClientRegistry(
			registry.WithEtcdV3(),
			registry.WithAddress(*registryAddr),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client: %v\n", err)
		os.Exit(1)
	}

	svc, err := greet.NewGreetService(cli)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create greet service: %v\n", err)
		os.Exit(1)
	}

	resp, err := svc.Greet(ctx, &greet.GreetRequest{Name: "Hello, Dubbo-Go!"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling Greet: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider:", resp.Greeting)
	if resp.Greeting != "Hello, Dubbo-Go!" {
		fmt.Fprintf(os.Stderr, "unexpected greet body: %q\n", resp.Greeting)
		os.Exit(1)
	}
}
