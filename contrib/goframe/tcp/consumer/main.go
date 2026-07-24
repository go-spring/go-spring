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
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/net/gsvc"
	"github.com/gogf/gf/v2/net/gtcp"
	"go-spring.org/spring/gs"
)

// Consumer holds the client-side settings injected from
// consumer/conf/app.properties. It never learns the provider's host:port: it
// resolves a live provider from the same etcd registry the provider published
// into, by the service name the provider registered under.
type Consumer struct {
	RegistryAddr string `value:"${goframe.consumer.registry.etcd:=127.0.0.1:2379}"`
	ServiceName  string `value:"${goframe.consumer.service.name:=goframe.tcp.echo}"`
}

func main() {
	// Consumer is not referenced by any other bean, so register it as a root
	// object and grab the handle to drive the one-shot call from runTest.
	bean := gs.Provide(&Consumer{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(bean.Interface().(*Consumer))
	}()

	// The consumer runs server-less: no HTTP server (disabled in
	// consumer/conf/app.properties) and no goframe server, so gs.Run() simply
	// blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest builds an etcd-backed gsvc.Registry pointing at the same endpoint
// the provider registered with, searches by service name to obtain a live
// Endpoint, and only then dials the raw TCP address.
//
// This is the same discover+dial pattern used by the WebSocket sibling, and
// for the same reason: goframe has no framework-level TCP client that
// understands gsvc://<name>. All non-HTTP/non-gRPC transports collapse to
// "Search, then hand the endpoint to a protocol-native client". That is the
// point of exercising gtcp here — the sibling http/grpc examples happen to
// hide this step behind their transport-specific discovery hooks, but the
// primitive underneath is identical.
func runTest(c *Consumer) {
	registry := etcdreg.New(c.RegistryAddr)
	// Setting the default registry is not strictly required for Search, but
	// keeps the wiring symmetric with the http and grpc siblings.
	gsvc.SetRegistry(registry)

	// Cap the whole discover+dial window so a lost provider doesn't hang
	// the smoke test forever.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	services, err := registry.Search(ctx, gsvc.SearchInput{Name: c.ServiceName})
	if err != nil {
		fmt.Fprintf(os.Stderr, "service search failed: %v\n", err)
		os.Exit(1)
	}
	if len(services) == 0 {
		fmt.Fprintf(os.Stderr, "no service instances found for %q\n", c.ServiceName)
		os.Exit(1)
	}
	endpoints := services[0].GetEndpoints()
	if len(endpoints) == 0 {
		fmt.Fprintf(os.Stderr, "service %q has no endpoints\n", c.ServiceName)
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

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
