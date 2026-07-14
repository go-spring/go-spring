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
	greet "go-spring.org/dubbo-go/dubbo/proto"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-dubbo"
)

// Consumer discovers the GreetService through the registry and calls it. The
// Dubbo client is the default client bean provided by starter-dubbo (built from
// ${spring.dubbo.client} + the top-level ${spring.dubbo.registries}), injected here
// the same way the redis example autowires *redis.Client into its Service bean.
type Consumer struct {
	Client *client.Client `autowire:"__default__"`
}

// Greet dials the GreetService by its Java-style interface name and invokes the
// Greet method. Because classic Dubbo has no generated stub (unlike the Triple
// sibling), the method name and argument list are passed as runtime values via
// the low-level Connection.CallUnary.
func (c *Consumer) Greet(ctx context.Context, name string) (string, error) {
	conn, err := c.Client.Dial(greet.GreetServiceInterface)
	if err != nil {
		return "", err
	}
	var resp string
	if err = conn.CallUnary(ctx, []any{name}, &resp, greet.MethodGreet); err != nil {
		return "", err
	}
	return resp, nil
}

func main() {
	// Consumer is not referenced by any other bean, so register it as a root
	// object and grab the handle to drive the one-shot call from runTest.
	svrBean := gs.Provide(&Consumer{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Consumer))
	}()

	// The consumer runs server-less: no HTTP server (disabled in the shared
	// conf/app.properties) and no Dubbo server (no ServiceRegister bean, so
	// starter-dubbo's server condition never fires), so gs.Run() simply blocks
	// until runTest sends SIGTERM.
	gs.Run()
}

// runTest exercises the Greet RPC end-to-end and asserts on the echoed value.
// On failure it exits(1); on success it sends SIGTERM so gs.Run() shuts down
// cleanly, making the process exit code the smoke-test result for check.sh.
func runTest(c *Consumer) {
	ctx := context.Background()

	want := "Hello, Dubbo-Go!"
	resp, err := c.Greet(ctx, want)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "error calling %s: %v", greet.MethodGreet, err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider:", resp)
	if resp != want {
		log.Errorf(ctx, log.TagAppDef, "unexpected greet body: %q", resp)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory to the module root (the parent of this
// consumer/ directory) so it loads the same conf/app.properties as the
// provider, regardless of the process launch path.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	moduleRoot := filepath.Dir(filepath.Dir(filename))
	if err := os.Chdir(moduleRoot); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
