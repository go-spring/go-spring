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
	greet "go-spring.org/dubbo-go/dubbo/proto"
)

// The consumer never learns the provider's host:port. It builds a client
// bound to the same etcd registry, asks for the GreetService by its Java-style
// interface name (com.example.GreetService, defined in proto/greet.go), and
// Dubbo resolves a live provider address from etcd, calls it, and we assert
// on the echo.
//
// Because this is the classic Dubbo protocol (TCP + Hessian2), the client is
// built with client.WithClientProtocolDubbo() and the call goes through a
// low-level Connection.CallUnary — there is no generated stub here, unlike
// the Triple sibling. The method name and argument list are passed as
// runtime values rather than compile-time-typed function calls.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	flag.Parse()

	ctx := context.Background()

	cli, err := client.NewClient(
		client.WithClientProtocolDubbo(),
		client.WithClientRegistry(
			registry.WithEtcdV3(),
			registry.WithAddress(*registryAddr),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client: %v\n", err)
		os.Exit(1)
	}

	conn, err := cli.Dial(greet.GreetServiceInterface)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial %s: %v\n", greet.GreetServiceInterface, err)
		os.Exit(1)
	}

	want := "Hello, Dubbo-Go!"
	var resp string
	if err := conn.CallUnary(ctx, []any{want}, &resp, greet.MethodGreet); err != nil {
		fmt.Fprintf(os.Stderr, "error calling %s: %v\n", greet.MethodGreet, err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider:", resp)
	if resp != want {
		fmt.Fprintf(os.Stderr, "unexpected greet body: %q\n", resp)
		os.Exit(1)
	}
}
