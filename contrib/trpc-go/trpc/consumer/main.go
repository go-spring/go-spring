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
	greet "go-spring.org/trpc-go/trpc/idl"
	_ "go-spring.org/trpc-go/trpc/trpcgs" // installs the tRPC → go-spring log bridge
	"trpc.group/trpc-go/trpc-go/client"
)

// This consumer dials the provider DIRECTLY by address using the "ip://" target
// scheme (no service registry/discovery in this first example — direct-connect
// keeps it dependency-free). It calls Greet once, self-asserts, then sends
// SIGTERM so gs.Run() shuts down cleanly, making the process exit code the
// smoke-test result.

// Consumer holds the client-side settings injected from
// consumer/conf/app.properties.
type Consumer struct {
	Target string `value:"${spring.trpc.consumer.target:=ip://127.0.0.1:8000}"`
}

func main() {
	bean := gs.Provide(&Consumer{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(bean.Interface().(*Consumer))
	}()

	// The consumer runs server-less: HTTP disabled in consumer/conf and no
	// trpcgs.ServiceRegister bean, so gs.Run() blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest calls the provider once and self-asserts, exiting non-zero on failure.
func runTest(c *Consumer) {
	proxy := greet.NewGreetServiceClientProxy(client.WithTarget(c.Target))

	want := "Hello, Go-Spring!"
	resp, err := proxy.Greet(context.Background(), &greet.GreetRequest{Name: "Go-Spring"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling Greet: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("response from provider: %s\n", resp.Greeting)
	if resp.Greeting != want {
		fmt.Fprintf(os.Stderr, "unexpected greeting: %q\n", resp.Greeting)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory to this consumer/ directory so it loads its
// own conf/app.properties regardless of the process launch path.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	dir := filepath.Dir(filename)
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
