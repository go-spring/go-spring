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
	"dubbo.apache.org/dubbo-go/v3/protocol/rest/config"
	greet "go-spring.org/dubbo-go/rest/idl"
	"go-spring.org/spring/gs"
	StarterDubbo "go-spring.org/starter-dubbo"
)

// ── REST protocol wiring ─────────────────────────────────────────────────────
//
// The REST protocol needs an HTTP routing table on both the provider and
// consumer side. This init installs the RestServiceConfigMap — a process-wide
// singleton that must be populated before the server/client starts. It is the
// one piece of REST that cannot be hidden behind a typed stub constructor;
// changing the path or verb on one side without updating the other silently
// produces 404s or 405s.

func init() {
	config.SetRestConsumerServiceConfigMap(map[string]*config.RestServiceConfig{
		greet.GreetServiceInterface: {
			// resty is the only client implementation shipped with dubbo-go v3.
			Client:   "resty",
			Produces: "application/json",
			Consumes: "application/json",
			RestMethodConfigsMap: map[string]*config.RestMethodConfig{
				greet.MethodGreet: {
					InterfaceName:  greet.GreetServiceInterface,
					MethodName:     greet.MethodGreet,
					Path:           greet.GreetPath,
					MethodType:     greet.GreetHTTPMethod,
					Produces:       "application/json",
					Consumes:       "application/json",
					QueryParamsMap: map[int]string{0: greet.GreetQueryName},
					Body:           -1,
				},
			},
		},
	})
}

// ── Typed stub (hand-written; not code-generated) ────────────────────────────
//
// For non-Triple protocols there is no code generator, so the typed wrapper
// lives here in the application. It wraps the raw *client.Client, Dial-ing
// once at construction time and reusing the same connection for every call,
// and exposes the same constructor shape as a Triple-generated
// NewXxxService so it slots into StarterDubbo.RegisterReference.

// GreetService is a typed wrapper around a Dubbo client connection for the
// GreetService. Business beans autowire *GreetService instead of the raw
// *client.Client.
type GreetService struct {
	conn *client.Connection
}

// Greet calls the remote Greet RPC.
func (s *GreetService) Greet(ctx context.Context, name string) (string, error) {
	var resp string
	if err := s.conn.CallUnary(ctx, []any{name}, &resp, greet.MethodGreet); err != nil {
		return "", err
	}
	return resp, nil
}

// NewGreetService constructs a *GreetService from the global *client.Client.
// The signature matches a Triple-generated constructor, so it can be passed
// directly to StarterDubbo.RegisterReference. It Dials the interface once at
// construction time; every call reuses the same connection.
func NewGreetService(cli *client.Client, opts ...client.ReferenceOption) (*GreetService, error) {
	conn, err := cli.Dial(greet.GreetServiceInterface, opts...)
	if err != nil {
		return nil, err
	}
	return &GreetService{conn: conn}, nil
}

// ── Business bean ────────────────────────────────────────────────────────────

// Consumer calls the GreetService discovered through the registry. It depends
// on the typed *GreetService stub (registered via RegisterReference below)
// rather than the raw *client.Client — the same layering real apps use.
type Consumer struct {
	Svc *GreetService `autowire:""`
}

func main() {
	// Register the typed GreetService stub as a bean. StarterDubbo.RegisterReference
	// wires the single global client and the per-reference config under
	// ${spring.dubbo.client.references.greet} into NewGreetService.
	// The REST-specific RestServiceConfigMap is installed in the init above;
	// everything else — registry discovery, protocol selection, timeout — flows
	// through the same config path as the other protocols.
	StarterDubbo.RegisterReference("greet", NewGreetService)

	// Consumer is not referenced by any other bean, so register it as a root
	// object and grab the handle to drive the one-shot call from runTest.
	bean := gs.Provide(&Consumer{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(bean.Interface().(*Consumer))
	}()

	// The consumer runs server-less: no HTTP server (disabled in
	// consumer/conf/app.properties) and no Dubbo server (this file registers
	// only a client), so gs.Run() simply blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest exercises the Greet RPC end-to-end via the autowired typed stub
// and asserts on the echoed value.
func runTest(c *Consumer) {
	ctx := context.Background()

	want := "Hello, Dubbo-Go!"
	resp, err := c.Svc.Greet(ctx, want)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling %s: %v\n", greet.MethodGreet, err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider:", resp)
	if resp != want {
		fmt.Fprintf(os.Stderr, "unexpected greet body: %q\n", resp)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
