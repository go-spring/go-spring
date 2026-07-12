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
	"github.com/cloudwego/kitex/transport"
	etcd "github.com/kitex-contrib/registry-etcd"
	echo "go-spring.org/kitex/protobuf/kitex_gen/echo"
	"go-spring.org/kitex/protobuf/kitex_gen/echo/echoservice"
)

// The provider was generated from a protobuf IDL, so a single provider serves
// BOTH protobuf transports on the same port: KitexProtobuf (Kitex's own
// protobuf-over-TTHeader payload, the default) and gRPC (protobuf over
// HTTP/2). The choice is made per client via client.WithTransportProtocol; the
// server sniffs the connection and dispatches accordingly.
//
// This consumer resolves the provider from etcd by service name and calls it
// once over each transport, asserting both, to prove the one provider speaks
// both protocols.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	serviceName := flag.String("service", "echo", "target Kitex service name")
	flag.Parse()

	// KitexProtobuf is the default when no transport protocol is specified.
	call(*registryAddr, *serviceName, "KitexProtobuf")
	// gRPC is selected via WithTransportProtocol(transport.GRPC).
	call(*registryAddr, *serviceName, "gRPC", client.WithTransportProtocol(transport.GRPC))
}

// call builds a client over the given transport, resolves a live provider from
// etcd, performs one Echo and self-asserts, exiting non-zero on any failure.
func call(registryAddr, serviceName, label string, extra ...client.Option) {
	r, err := etcd.NewEtcdResolver([]string{registryAddr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] failed to create etcd resolver: %v\n", label, err)
		os.Exit(1)
	}

	opts := append([]client.Option{client.WithResolver(r)}, extra...)
	cli, err := echoservice.NewClient(serviceName, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] failed to create client: %v\n", label, err)
		os.Exit(1)
	}

	want := "Hello, Kitex!"
	resp, err := cli.Echo(context.Background(), &echo.EchoRequest{Message: want})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] error calling Echo: %v\n", label, err)
		os.Exit(1)
	}

	fmt.Printf("[%s] response from discovered provider: %s\n", label, resp.Message)
	if resp.Message != want {
		fmt.Fprintf(os.Stderr, "[%s] unexpected echo body: %q\n", label, resp.Message)
		os.Exit(1)
	}
}
