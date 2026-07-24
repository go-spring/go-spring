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

	"dubbo.apache.org/dubbo-go/v3/client"
	_ "dubbo.apache.org/dubbo-go/v3/imports"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterDubbo "go-spring.org/starter-dubbo"
	greet "go-spring.org/starter-dubbo/example/idl/proto"
)

func init() {
	// Register the GreetProvider as a Dubbo service via starter-dubbo's helper.
	// RegisterService binds ${spring.dubbo.provider.services.greet} (per-service
	// overrides, including per-method tuning under .methods) and turns it into
	// dubbo-go server.ServiceOption passed to the generated handler - so the
	// service is exported with that config without the server knowing about
	// greet.GreetServiceHandler. T is inferred from the generated handler's
	// parameter type (the handler interface); the concrete &GreetProvider{} is
	// converted to that interface at the call site (a Go generics limitation -
	// a concrete value can't match a type parameter inferred as an interface). SimpleDubboServer collects this ServiceRegister
	// bean (and any others) and invokes it when the server starts.
	StarterDubbo.RegisterService("greet", greet.RegisterGreetServiceHandler,
		greet.GreetServiceHandler(&GreetProvider{}))
}

// GreetProvider implements the GreetServiceHandler interface generated from
// greet.proto.
type GreetProvider struct{}

// Greet echoes the request name back as the greeting, giving the client a
// deterministic value to assert on.
func (s *GreetProvider) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
	return &greet.GreetResponse{Greeting: req.Name}, nil
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	// The built-in HTTP server is disabled via conf/app.properties; gs.Run()
	// starts only the Dubbo server registered by starter-dubbo. No inline
	// overrides so the example matches production wiring.
	gs.Run()
}

// runTest dials the Dubbo server, exercises the Greet RPC end-to-end and
// asserts on the result. On failure it exits(1); on success it sends SIGTERM
// so gs.Run() shuts down cleanly.
func runTest() {
	ctx := context.Background()

	cli, err := client.NewClient(client.WithClientURL("127.0.0.1:20000"))
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "failed to create client: %v", err)
		os.Exit(1)
	}

	svc, err := greet.NewGreetService(cli)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "failed to create greet service: %v", err)
		os.Exit(1)
	}

	resp, err := svc.Greet(ctx, &greet.GreetRequest{Name: "Hello, Dubbo-Go!"})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error calling Greet: %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", resp.Greeting)
	if resp.Greeting != "Hello, Dubbo-Go!" {
		log.Errorf(ctx, log.TagAppDef, "unexpected greet body: %q", resp.Greeting)
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
