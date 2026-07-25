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

// Package main is the observability example for starter-thrift.
//
// It starts a Thrift server with tracing and metrics middleware enabled,
// sends a batch of RPC calls to generate observability signals, then
// verifies traces reach Jaeger and self-exits. Traces are exported via
// OTLP/gRPC to Jaeger (docker-compose) which accepts OTLP natively
// with COLLECTOR_OTLP_ENABLED=true.
//
// Run with -manual to keep the server running for interactive exploration.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-otel"
	_ "go-spring.org/starter-thrift"

	"go-spring.org/starter-thrift/example/idl/proto"
)

func init() {
	// Register the Echo controller and the wrapped thrift.TProcessor bean.
	gs.Provide(&Controller{})
	gs.Provide(func(c *Controller) thrift.TProcessor {
		return proto.NewEchoServiceProcessor(c)
	})
}

// Controller is the EchoService handler. Echo returns the request message unchanged.
type Controller struct{}

// Echo satisfies proto.EchoService.
func (c *Controller) Echo(ctx context.Context, req *proto.EchoRequest) (*proto.EchoResponse, error) {
	log.Infof(ctx, log.TagAppDef, "handling Echo: %s", req.Message)
	return &proto.EchoResponse{Message: req.Message}, nil
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	if !*manual {
		go func() {
			time.Sleep(700 * time.Millisecond)
			runTest()
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Open Jaeger at http://localhost:16686")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

func runTest() {
	ctx := context.Background()

	socket := thrift.NewTSocketConf(":9292", nil)
	transport := thrift.NewTFramedTransportConf(socket, nil)
	defer transport.Close()

	protocolFactory := thrift.NewTCompactProtocolFactoryConf(nil)
	client := proto.NewEchoServiceClientFactory(transport, protocolFactory)

	if err := transport.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "Error opening transport:", err)
		os.Exit(1)
	}

	// Generate traffic to produce traces.
	for i := 0; i < 20; i++ {
		_, err := client.Echo(ctx, &proto.EchoRequest{Message: "world"})
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error calling Echo:", err)
			os.Exit(1)
		}
	}
	fmt.Println("Sent 20 Echo RPCs")

	// Wait for collector to flush.
	time.Sleep(3 * time.Second)

	// Verify traces appear in Jaeger.
	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=thrift-otel-example&limit=1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Jaeger API request failed:", err)
		os.Exit(1)
	}
	if !strings.Contains(traces, `"data":[`) {
		fmt.Fprintln(os.Stderr, "no traces found in Jaeger for service 'thrift-otel-example'")
		os.Exit(1)
	}
	fmt.Println("OK: traces found in Jaeger for service 'thrift-otel-example'")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

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
