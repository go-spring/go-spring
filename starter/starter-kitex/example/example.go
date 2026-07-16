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

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/server"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterKitex "go-spring.org/starter-kitex"
	echo "go-spring.org/starter-kitex/example/kitex_gen/echo"
	"go-spring.org/starter-kitex/example/kitex_gen/echo/echoservice"
)

func init() {
	// Provide a StarterKitex.ServiceRegister bean that binds EchoServiceImpl to
	// a raw Kitex server via the generated echoservice.RegisterService.
	// starter-kitex's SimpleKitexServer depends only on this function type, so
	// the concrete service is wired here without the server knowing about the
	// generated echo service.
	gs.Provide(func() StarterKitex.ServiceRegister {
		return func(svr server.Server) error {
			return echoservice.RegisterService(svr, &EchoServiceImpl{})
		}
	})
}

// EchoServiceImpl implements the EchoService defined in echo.thrift.
type EchoServiceImpl struct{}

// Echo returns the request message unchanged, giving the client a
// deterministic value to assert on.
func (s *EchoServiceImpl) Echo(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
	return &echo.EchoResponse{Message: req.Message}, nil
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

// runTest dials the server started by starter-kitex and asserts a round-trip.
// The example runs registry-free (conf leaves registry.etcd empty), so the
// client reaches the provider directly by host:port instead of via etcd.
func runTest() {
	ctx := context.Background()

	cli, err := echoservice.NewClient("echo", client.WithHostPorts(":8888"))
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "failed to create client: %v", err)
		os.Exit(1)
	}

	resp, err := cli.Echo(ctx, &echo.EchoRequest{Message: "Hello, Kitex!"})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "error calling Echo: %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", resp.Message)

	if resp.Message != "Hello, Kitex!" {
		log.Errorf(ctx, log.TagAppDef, "unexpected echo body: %q", resp.Message)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

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
