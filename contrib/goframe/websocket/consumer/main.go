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
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/net/gsvc"
	"github.com/gorilla/websocket"
	"go-spring.org/spring/gs"
)

// Consumer holds the client-side settings injected from
// consumer/conf/app.properties. It never learns the provider's host:port: it
// resolves a live provider from the same etcd registry the provider published
// into, by the service name the provider registered under.
type Consumer struct {
	RegistryAddr string `value:"${goframe.consumer.registry.etcd:=127.0.0.1:2379}"`
	ServiceName  string `value:"${goframe.consumer.service.name:=goframe.websocket.echo}"`
	Path         string `value:"${goframe.consumer.websocket.path:=/echo}"`
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
// Endpoint, and only then dials the upgrade URL with gorilla/websocket.
//
// Why the two-step discover+dial (vs. the http sibling's one-liner
// `g.Client().Discovery(reg).Get(ctx, "http://<svc>/hello")`): goframe's
// gclient discovery middleware is HTTP-only — it rewrites r.URL.Host to a real
// address inside its RoundTripper. There is no equivalent ws-aware client in
// goframe today, so we resolve the endpoint via the gsvc Discovery API
// directly and hand the resulting host:port to gorilla/websocket.
//
// The upshot: registration still flows through goframe's gsvc (via the
// provider's ghttp.Server), and discovery still comes from the same etcd
// prefix; only the dial changes.
func runTest(c *Consumer) {
	// etcdreg.Registry implements both gsvc.Registrar (used by the provider)
	// and gsvc.Discovery (used here). Setting the default registry is not
	// strictly required for Search, but keeps the wiring symmetric with the
	// http sibling and with any future middleware that reads gsvc.GetRegistry().
	registry := etcdreg.New(c.RegistryAddr)
	gsvc.SetRegistry(registry)

	// Cap the whole handshake window so a lost provider doesn't hang the
	// smoke test forever.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ask etcd for services registered under svcName. The registry watches
	// this prefix and returns whatever is currently alive.
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

	// Build the WebSocket URL from the resolved endpoint. gorilla/websocket
	// speaks the standard ws:// scheme; the upgrade goes over plain TCP.
	// Endpoint.Host() may be empty when the provider bound to :port (in
	// which case ghttp registers the local IP into etcd, so this branch is
	// defensive rather than expected).
	host := ep.Host()
	if host == "" {
		host = "127.0.0.1"
	}
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", host, ep.Port()), Path: c.Path}
	fmt.Println("Dialing discovered provider:", u.String())

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "websocket dial failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// One round-trip: send a message, expect the same bytes echoed back.
	// The provider's /echo handler is a straight ReadMessage/WriteMessage
	// loop, so a mismatch here would be a real regression.
	want := "Hello, GoFrame WebSocket!"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(want)); err != nil {
		fmt.Fprintf(os.Stderr, "websocket write failed: %v\n", err)
		os.Exit(1)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "websocket read failed: %v\n", err)
		os.Exit(1)
	}

	got := string(data)
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
