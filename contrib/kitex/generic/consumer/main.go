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
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/pkg/generic"
	etcd "github.com/kitex-contrib/registry-etcd"
)

// The consumer here is intentionally different from the ../thrift and
// ../protobuf siblings: it never imports kitex_gen and never sees a typed
// EchoRequest / EchoResponse struct.
//
// Instead it uses Kitex's JSON generic invocation:
//
//   - generic.NewThriftFileProvider parses idl/echo.thrift AT RUNTIME.
//   - generic.JSONThriftGeneric builds a codec that marshals a JSON string
//     argument into the standard Thrift wire format defined by that IDL, and
//     the response back into a JSON string.
//   - genericclient.NewClient produces a client whose only call surface is
//     GenericCall(ctx, methodName, jsonBody).
//
// The wire format on the network is exactly the same TTHeader/Thrift a
// code-generated client would send, so this dials the same unmodified server
// that the ../thrift subproject uses — the difference is entirely on the
// client side. This is the capability that neither the Thrift subproject
// (typed stubs on both sides) nor the Protobuf subproject (typed stubs; two
// wire protocols selected per client at call time) demonstrates: talking to a
// Kitex service with only the IDL, no generated code.
//
// Real-world use cases: API gateways that proxy REST/JSON to internal Thrift
// services, admin tools that must call arbitrary services without rebuilding,
// test harnesses, and cross-language bridges.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	serviceName := flag.String("service", "echo-generic", "target Kitex service name")
	idlPath := flag.String("idl", "idl/echo.thrift", "path to the Thrift IDL file to parse at runtime")
	flag.Parse()

	// 1. Parse the IDL from disk. This replaces the code-generation step: no
	//    Go structs are produced, but the codec learns the shape of every
	//    method and struct in the file.
	p, err := generic.NewThriftFileProvider(*idlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load thrift IDL %s: %v\n", *idlPath, err)
		os.Exit(1)
	}

	// 2. Wrap the parsed IDL in a JSON<->Thrift codec.
	g, err := generic.JSONThriftGeneric(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build json-thrift generic codec: %v\n", err)
		os.Exit(1)
	}

	// 3. Build a generic client bound to the same etcd registry the provider
	//    published into. Discovery works exactly as in the sibling examples;
	//    the only difference is that this client has no method-typed handles.
	r, err := etcd.NewEtcdResolver([]string{*registryAddr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd resolver: %v\n", err)
		os.Exit(1)
	}
	cli, err := genericclient.NewClient(*serviceName, g, client.WithResolver(r))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create generic client: %v\n", err)
		os.Exit(1)
	}

	// 4. Invoke by method name with a JSON body. No generated argument type
	//    is used anywhere; the codec builds the Thrift request from the JSON
	//    according to the IDL, and returns the response as a JSON string.
	//    For JSON generic, the payload is the *flat* request struct JSON
	//    (e.g. `{"message":"hi"}`) — NOT wrapped by the Thrift argument name.
	want := "Hello, Kitex!"
	reqJSON := fmt.Sprintf(`{"message":%q}`, want)
	respAny, err := cli.GenericCall(context.Background(), "Echo", reqJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling Echo: %v\n", err)
		os.Exit(1)
	}

	// The generic JSON codec returns the response as a JSON string.
	respJSON, ok := respAny.(string)
	if !ok {
		fmt.Fprintf(os.Stderr, "unexpected response type %T: %v\n", respAny, respAny)
		os.Exit(1)
	}
	fmt.Println("Raw JSON response from discovered provider:", respJSON)

	// Parse the JSON just enough to self-assert on the round-tripped body.
	var parsed struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(respJSON), &parsed); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse response JSON: %v\n", err)
		os.Exit(1)
	}
	if parsed.Message != want {
		fmt.Fprintf(os.Stderr, "unexpected echo body: %q\n", parsed.Message)
		os.Exit(1)
	}
	fmt.Println("Generic call round-trip OK:", parsed.Message)
}
