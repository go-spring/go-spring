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
	echo "go-spring.org/kitex/kitex_gen/echo"
	"go-spring.org/kitex/kitex_gen/echo/echoservice"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
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
	// starts only the Kitex server registered in server.go. No inline
	// overrides so the example matches production wiring.
	gs.Run()
}

// runTest dials the Kitex server, exercises the Echo RPC end-to-end and
// asserts on the result. On failure it exits(1); on success it sends SIGTERM
// so gs.Run() shuts down cleanly.
func runTest() {
	ctx := context.Background()

	cli, err := echoservice.NewClient("echo", client.WithHostPorts(":8888"))
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "failed to create client: %v", err)
		os.Exit(1)
	}

	resp, err := cli.Echo(ctx, &echo.EchoRequest{Message: "Hello, Kitex!"})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error calling Echo: %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", resp.Message)
	if resp.Message != "Hello, Kitex!" {
		log.Errorf(ctx, log.TagAppDef, "unexpected echo body: %q", resp.Message)
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
