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

	_ "dubbo.apache.org/dubbo-go/v3/imports"
	"go-spring.org/log"
	greet "go-spring.org/registry/etcd/idl"
	"go-spring.org/spring/gs"
	"go-spring.org/starter-dubbo"
)

// Consumer discovers the GreetService through the registry and calls it. Rather
// than holding the raw *client.Client and rebuilding the stub on every call, it
// depends on the greet.GreetService stub bean (registered in main below via
// StarterDubbo.RegisterReference, which itself is built from starter-dubbo's
// named "greet" client) - the same layering real apps use: a business bean
// depends on the typed RPC stub, the stub depends on the client.
type Consumer struct {
	Svc greet.GreetService `autowire:""`
}

// Greet invokes the Greet RPC through the autowired typed stub. The stub is
// built once (when the bean is wired), not on every call.
func (c *Consumer) Greet(ctx context.Context, name string) (string, error) {
	resp, err := c.Svc.Greet(ctx, &greet.GreetRequest{Name: name})
	if err != nil {
		return "", err
	}
	return resp.Greeting, nil
}

func main() {
	// Register the Triple-generated GreetService stub as a bean (instead of
	// rebuilding it on every call). StarterDubbo.RegisterReference wires the
	// named "greet" client and the per-reference config under
	// ${spring.dubbo.references.greet} into the stub constructor - a starter
	// helper reusable for any stub. The stub bean is then autowired into
	// Consumer below (business bean -> RPC stub -> client).
	StarterDubbo.RegisterReference("greet", greet.NewGreetService)

	svrBean := gs.Provide(&Consumer{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Consumer))
	}()

	// The consumer runs server-less: no HTTP server and no Dubbo server, so
	// gs.Run() simply blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest exercises the Greet RPC end-to-end and asserts on the echoed value.
// scripts/smoke-test.sh greps the printed line, so keep its wording stable.
func runTest(c *Consumer) {
	ctx := context.Background()
	want := "Hello, Dubbo-Go!"
	resp, err := c.Greet(ctx, want)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "error calling Greet: %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from discovered provider:", resp)
	if resp != want {
		log.Errorf(ctx, log.TagAppDef, "unexpected greet body: %q", resp)
		os.Exit(1)
	}
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init pins the working directory to this consumer/ directory so it loads its
// own conf/app.properties regardless of the process launch path.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
	workDir, _ := os.Getwd()
	fmt.Println(workDir)
}
