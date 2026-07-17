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
	greet "go-spring.org/dubbo-go/dubbo/idl"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-dubbo"
)

// Consumer discovers the GreetService through the registry and calls it. The
// Dubbo client is the default client bean provided by starter-dubbo (built from
// ${spring.dubbo.client} + the top-level ${spring.dubbo.registries}), injected here
// the same way the redis example autowires *redis.Client into its Service bean.
type Consumer struct {
	Client *client.Client `autowire:""`
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

	// The consumer runs server-less: no HTTP server (disabled in
	// consumer/conf/app.properties) and no Dubbo server (no ServiceRegister
	// bean, so starter-dubbo's server condition never fires), so gs.Run() simply
	// blocks until runTest sends SIGTERM.
	gs.Run()
}

// greetCalls is how many RPCs runTest issues. A single call is enough to prove
// wiring, but it leaves the observability backends nearly empty (one span, one
// log line, a counter of 1). Issuing a batch gives Prometheus a counter worth
// graphing, Jaeger a handful of traces to browse, and Loki several structured
// log lines to query — so the manual verification steps in the README show real
// data, not a lone sample.
const greetCalls = 20

// runTest exercises the Greet RPC end-to-end and asserts on the echoed value.
// It first makes the canonical call (whose printed line scripts/smoke-test.sh greps for),
// then a batch of further calls to populate the observability stack. On any
// failure it exits(1); on success it sends SIGTERM so gs.Run() shuts down
// cleanly, making the process exit code the smoke-test result for scripts/smoke-test.sh.
func runTest(c *Consumer) {
	ctx := context.Background()

	// Canonical call: deterministic payload the provider echoes back. scripts/smoke-test.sh
	// asserts on the printed line below, so keep its wording stable.
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

	// Batch of additional calls to give the observability backends real volume.
	// Each call is a fresh trace (span "Greet"), bumps the provider's request
	// counters, and emits a structured INFO log line that Promtail ships to Loki.
	for i := 1; i <= greetCalls; i++ {
		name := fmt.Sprintf("Dubbo-Go caller #%d", i)
		got, err := c.Greet(ctx, name)
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "error on greet #%d: %v", i, err)
			os.Exit(1)
		}
		if got != name {
			log.Errorf(ctx, log.TagAppDef, "greet #%d echoed %q, want %q", i, got, name)
			os.Exit(1)
		}
		log.Infof(ctx, log.TagAppDef, "greet #%d ok: %q", i, got)
	}
	fmt.Printf("Sent %d greetings (1 canonical + %d batch)\n", greetCalls+1, greetCalls)

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory to this consumer/ directory so it loads its
// own conf/app.properties (consumer/conf/app.properties) regardless of the
// process launch path. The provider does the same with its own conf, so the two
// no longer share a file or need env-var overrides to avoid colliding.
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
