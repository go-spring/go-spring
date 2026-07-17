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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	goframegrpc "go-spring.org/starter-goframe/grpc"
	"go-spring.org/starter-goframe/grpc/example/idl/echo"
)

// EchoServiceImpl implements the EchoService gRPC interface defined in
// idl/echo.proto. Echo returns the request message unchanged, giving the client
// a deterministic value to assert on.
type EchoServiceImpl struct {
	echo.UnimplementedEchoServiceServer
}

func (s *EchoServiceImpl) Echo(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
	return &echo.EchoResponse{Message: req.Message}, nil
}

func init() {
	// Provide a ServiceRegister bean that binds EchoServiceImpl onto the raw
	// *grpc.Server via the generated echo.RegisterEchoServiceServer. The starter
	// depends only on this function type, so the concrete service is wired here.
	gs.Provide(func() goframegrpc.ServiceRegister {
		return func(s grpc.ServiceRegistrar) {
			echo.RegisterEchoServiceServer(s, &EchoServiceImpl{})
		}
	})
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

func runTest() {
	// No etcd configured, so dial the provider directly on its bind address.
	conn, err := grpc.NewClient("127.0.0.1:8001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial failed:", err)
		os.Exit(1)
	}
	defer conn.Close()

	cli := echo.NewEchoServiceClient(conn)
	resp, err := cli.Echo(context.Background(), &echo.EchoRequest{Message: "hello"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "echo call failed:", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", resp.Message)

	if resp.Message != "hello" {
		fmt.Fprintln(os.Stderr, "unexpected echo message:", resp.Message)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory to this source file's directory so the process
// loads its own conf/app.properties regardless of the launch path.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
}
