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

// Package main is the observability example for starter-goframe/grpc.
//
// It starts a GoFrame gRPC server with tracing and metrics interceptors
// enabled, sends a batch of RPC calls to generate observability signals,
// then verifies traces reach Jaeger and self-exits. Traces are exported
// via OTLP/gRPC to Jaeger (docker-compose) which accepts OTLP natively
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

	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	goframegrpc "go-spring.org/starter-goframe/grpc"
	echo "go-spring.org/starter-goframe/grpc/example/idl/echo"
)

// EchoServiceImpl implements the EchoService gRPC interface.
type EchoServiceImpl struct {
	echo.UnimplementedEchoServiceServer
}

func (s *EchoServiceImpl) Echo(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
	return &echo.EchoResponse{Message: req.Message}, nil
}

func init() {
	gs.Provide(func() goframegrpc.ServiceRegister {
		return func(s grpc.ServiceRegistrar) {
			echo.RegisterEchoServiceServer(s, &EchoServiceImpl{})
		}
	})
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
	conn, err := grpc.NewClient("127.0.0.1:8001",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial failed:", err)
		os.Exit(1)
	}
	defer conn.Close()

	cli := echo.NewEchoServiceClient(conn)

	// Generate traffic to produce traces.
	for i := 0; i < 20; i++ {
		_, err := cli.Echo(context.Background(), &echo.EchoRequest{Message: "world"})
		if err != nil {
			fmt.Fprintln(os.Stderr, "echo call failed:", err)
			os.Exit(1)
		}
	}
	fmt.Println("Sent 20 Echo RPCs")

	// Wait for collector to flush.
	time.Sleep(3 * time.Second)

	// Verify traces appear in Jaeger.
	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=goframe-grpc-otel-example&limit=1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Jaeger API request failed:", err)
		os.Exit(1)
	}
	if !strings.Contains(traces, `"data":[`) {
		fmt.Fprintln(os.Stderr, "no traces found in Jaeger for service 'goframe-grpc-otel-example'")
		os.Exit(1)
	}
	fmt.Println("OK: traces found in Jaeger for service 'goframe-grpc-otel-example'")

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