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
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/net/gsvc"
	"github.com/gogf/gf/v2/net/gtcp"
)

// The consumer never learns the provider's host:port. It builds an
// etcd-backed gsvc.Registry pointing at the same endpoint the provider
// registered with, searches by service name to obtain a live Endpoint, and
// only then dials the raw TCP address.
//
// This is the same discover+dial pattern used by the WebSocket sibling, and
// for the same reason: goframe has no framework-level TCP client that
// understands gsvc://<name>. All non-HTTP/non-gRPC transports collapse to
// "Search, then hand the endpoint to a protocol-native client". That is the
// point of exercising gtcp here — the sibling http/grpc examples happen to
// hide this step behind their transport-specific discovery hooks, but the
// primitive underneath is identical.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	svcName := flag.String("service", "goframe.tcp.echo", "service name registered by the provider")
	flag.Parse()

	registry := etcdreg.New(*registryAddr)
	// Setting the default registry is not strictly required for Search, but
	// keeps the wiring symmetric with the http and grpc siblings.
	gsvc.SetRegistry(registry)

	// Cap the whole discover+dial window so a lost provider doesn't hang
	// the smoke test forever.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	services, err := registry.Search(ctx, gsvc.SearchInput{Name: *svcName})
	if err != nil {
		fmt.Fprintf(os.Stderr, "service search failed: %v\n", err)
		os.Exit(1)
	}
	if len(services) == 0 {
		fmt.Fprintf(os.Stderr, "no service instances found for %q\n", *svcName)
		os.Exit(1)
	}
	endpoints := services[0].GetEndpoints()
	if len(endpoints) == 0 {
		fmt.Fprintf(os.Stderr, "service %q has no endpoints\n", *svcName)
		os.Exit(1)
	}
	ep := endpoints[0]
	addr := net.JoinHostPort(ep.Host(), strconv.Itoa(ep.Port()))
	fmt.Println("Dialing discovered provider:", addr)

	// gtcp.NewNetConn is the "just give me a net.Conn" entry point. gtcp
	// also has NewConn returning a *gtcp.Conn, but for a single-frame echo
	// the plain net.Conn is enough and keeps the read/write path readable.
	conn, err := gtcp.NewNetConn(addr, 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcp dial failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// One round-trip: send a newline-terminated frame, read the echoed
	// bytes, trim the delimiter, assert. The provider handler is a straight
	// bufio.ReadBytes('\n')/Write echo, so a mismatch would be a real
	// regression.
	want := "Hello, GoFrame TCP!"
	if _, err := conn.Write([]byte(want + "\n")); err != nil {
		fmt.Fprintf(os.Stderr, "tcp write failed: %v\n", err)
		os.Exit(1)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcp read failed: %v\n", err)
		os.Exit(1)
	}
	got := strings.TrimRight(string(buf[:n]), "\r\n")

	fmt.Println("Response from discovered provider:", got)
	if got != want {
		fmt.Fprintf(os.Stderr, "unexpected echo body: %q\n", got)
		os.Exit(1)
	}
}
