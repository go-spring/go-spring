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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	v1 "go-spring.org/go-kratos/api/helloworld/v1"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	// Blank imports trigger each layer's init() so its beans register with the
	// Go-Spring container. This replaces the scaffold's wire_gen.go wiring.
	_ "go-spring.org/go-kratos/internal/biz"
	_ "go-spring.org/go-kratos/internal/data"
	_ "go-spring.org/go-kratos/internal/server"
	_ "go-spring.org/go-kratos/internal/service"
)

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	// The built-in HTTP server is disabled via conf/app.properties; gs.Run()
	// starts only the kratos HTTP and gRPC servers registered in
	// internal/server. No inline overrides, so the example matches how a real
	// Go-Spring service is wired.
	gs.Run()
}

// runTest exercises both the HTTP and gRPC Greeter endpoints end-to-end and
// asserts on the results. On failure it exits(1); on success it sends SIGTERM
// so gs.Run() shuts down cleanly.
func runTest() {
	ctx := context.Background()

	// HTTP: GET /helloworld/{name} -> {"message":"Hello Kratos"}
	resp, err := http.Get("http://127.0.0.1:8000/helloworld/Kratos")
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "HTTP request failed: %v", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var httpReply v1.HelloReply
	if err = json.Unmarshal(body, &httpReply); err != nil {
		log.Errorf(ctx, log.TagAppDef, "failed to decode HTTP reply %q: %v", body, err)
		os.Exit(1)
	}
	fmt.Println("HTTP response from server:", httpReply.Message)
	if httpReply.Message != "Hello Kratos" {
		log.Errorf(ctx, log.TagAppDef, "unexpected HTTP reply: %q", httpReply.Message)
		os.Exit(1)
	}

	// gRPC: SayHello -> "Hello Kratos"
	conn, err := kgrpc.DialInsecure(ctx, kgrpc.WithEndpoint("127.0.0.1:9000"))
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "failed to dial gRPC server: %v", err)
		os.Exit(1)
	}
	defer conn.Close()
	grpcReply, err := v1.NewGreeterClient(conn).SayHello(ctx, &v1.HelloRequest{Name: "Kratos"})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "gRPC SayHello failed: %v", err)
		os.Exit(1)
	}
	fmt.Println("gRPC response from server:", grpcReply.Message)
	if grpcReply.Message != "Hello Kratos" {
		log.Errorf(ctx, log.TagAppDef, "unexpected gRPC reply: %q", grpcReply.Message)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory to the directory where this source file
// resides, so relative config loading (conf/app.properties) works regardless
// of the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
