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
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/spring/gs"
	StarterTrpc "go-spring.org/starter-trpc"
	greet "go-spring.org/starter-trpc/example/idl"
	"trpc.group/trpc-go/trpc-go/client"
	trpclog "trpc.group/trpc-go/trpc-go/log"
	"trpc.group/trpc-go/trpc-go/server"
)

func init() {
	// Provide a StarterTrpc.ServiceRegister bean that binds GreetServiceImpl onto
	// the tRPC server via the generated greet.RegisterGreetServiceService.
	// SimpleTrpcServer depends only on this function type, so the concrete
	// service is wired here without the adapter ever knowing about GreetService.
	gs.Provide(func() StarterTrpc.ServiceRegister {
		return func(s *server.Server) {
			greet.RegisterGreetServiceService(s, &GreetServiceImpl{})
		}
	})
}

// GreetServiceImpl implements the GreetService defined in greet.proto.
type GreetServiceImpl struct{}

// Greet returns a greeting for the request name. The trpclog.Infof call flows
// through the go-spring log bridge (see starter-trpc/logbridge.go), so the
// framework log line is written by go-spring's log pipeline.
func (s *GreetServiceImpl) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
	trpclog.Infof("greet request: %s", req.Name)
	return &greet.GreetResponse{Greeting: "Hello, " + req.Name + "!"}, nil
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	gs.Run()
}

// runTest dials the server started by starter-trpc and asserts a round-trip.
// The example is direct-connect (no service registry): the client reaches the
// provider by address via the "ip://" target scheme instead of by discovery.
func runTest() {
	proxy := greet.NewGreetServiceClientProxy(client.WithTarget("ip://127.0.0.1:8000"))

	want := "Hello, Go-Spring!"
	resp, err := proxy.Greet(context.Background(), &greet.GreetRequest{Name: "Go-Spring"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling Greet: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("response from server: %s\n", resp.Greeting)
	if resp.Greeting != want {
		fmt.Fprintf(os.Stderr, "unexpected greeting: %q\n", resp.Greeting)
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
