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
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	_ "dubbo.apache.org/dubbo-go/v3/imports"
	"dubbo.apache.org/dubbo-go/v3/protocol/rest/config"
	"dubbo.apache.org/dubbo-go/v3/registry"
	greet "go-spring.org/dubbo-go/rest/proto"
)

// init installs the RestServiceConfig map on the consumer side. The
// dubbo-go REST protocol resolves the map by `bean.name` on the URL. On
// the client the id defaults to `ref.InterfaceName` when the Dial call
// receives only an interface string (no ServiceInfo, no service struct),
// which is our case — so we key the map by the Java-style interface name
// instead of the provider-side struct name.
//
// It is a legitimate downside of REST vs. its siblings (Triple /
// classic-Dubbo / JSON-RPC), which need no such client-side registration.
// Populating this map is a build-time coupling to the provider's URL
// layout; changing the path or verb on one side without updating the other
// silently produces 404s or 405s.
func init() {
	config.SetRestConsumerServiceConfigMap(map[string]*config.RestServiceConfig{
		greet.GreetServiceInterface: {
			// resty is the only client implementation shipped with dubbo-go v3.
			Client:   "resty",
			Produces: "application/json",
			Consumes: "application/json",
			RestMethodConfigsMap: map[string]*config.RestMethodConfig{
				greet.MethodGreet: {
					InterfaceName:  greet.GreetServiceInterface,
					MethodName:     greet.MethodGreet,
					Path:           greet.GreetPath,
					MethodType:     greet.GreetHTTPMethod,
					Produces:       "application/json",
					Consumes:       "application/json",
					QueryParamsMap: map[int]string{0: greet.GreetQueryName},
					Body:           -1,
				},
			},
		},
	})
}

// The consumer never learns the provider's host:port. It builds a client
// bound to the same etcd registry, asks for the GreetService by its Java-style
// interface name (com.example.GreetService, defined in proto/greet.go), and
// Dubbo resolves a live provider address from etcd, calls it, and we assert
// on the echo.
//
// Because this is the REST protocol, dubbo-go has no dedicated
// client.WithClientProtocolREST() shortcut; we instead pass the generic
// client.WithProtocol(constant.RESTProtocol) as a ReferenceOption on Dial,
// which sets Reference.Protocol = "rest" so the registry-directory layer
// filters etcd URLs down to the rest:// entries.
//
// The call itself still flows through Connection.CallUnary — the framework
// hides the fact that the wire is `GET /greet?name=...` behind the same
// reflective invocation shape used by the classic-Dubbo and JSON-RPC
// siblings.
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

	// Pin the protocol at Dial time. Passing it through WithProtocol here
	// (rather than at NewClient time) matches dubbo-go's own factoring for
	// protocols that lack a WithClientProtocolXxx shortcut.
	conn, err := cli.Dial(greet.GreetServiceInterface,
		client.WithProtocol(constant.RESTProtocol))
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
