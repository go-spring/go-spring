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
	"strings"
	"syscall"
	"time"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/gsvc"
	"go-spring.org/spring/gs"
)

// Consumer holds the client-side settings injected from
// consumer/conf/app.properties. It never learns the provider's host:port: it
// resolves a live provider from the same etcd registry the provider published
// into, by the service name the provider registered under.
type Consumer struct {
	RegistryAddr string `value:"${goframe.consumer.registry.etcd:=127.0.0.1:2379}"`
	ServiceName  string `value:"${goframe.consumer.service.name:=goframe.hello}"`
}

func main() {
	// Consumer is not referenced by any other bean, so register it as a root
	// object and grab the handle to drive the one-shot call from runTest.
	bean := gs.Provide(&Consumer{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(bean.Interface().(*Consumer))
	}()

	// The consumer runs server-less: no HTTP server (disabled in
	// consumer/conf/app.properties) and no goframe server, so gs.Run() simply
	// blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest builds a gclient bound to the etcd registry, resolves the HTTP
// endpoint by service name, calls it and asserts on the response body. On
// success it sends SIGTERM so gs.Run() shuts down cleanly, making the process
// exit code the smoke-test result for scripts/smoke-test.sh.
func runTest(c *Consumer) {
	// Setting the global registry gives gclient a Discovery implementation via
	// gsvc.GetRegistry(); we still hand it to the client explicitly below,
	// which is closer to how registry wiring reads in the dubbo-go consumer.
	registry := etcdreg.New(c.RegistryAddr)
	gsvc.SetRegistry(registry)

	ctx := context.Background()

	// Discovery(registry) turns http://<svcName>/<path> into an etcd lookup;
	// r.URL.Host in gclient's discovery middleware is treated as a service
	// name, not a network host.
	url := fmt.Sprintf("http://%s/hello", c.ServiceName)
	resp, err := g.Client().Discovery(registry).Get(ctx, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Close()

	text := strings.TrimSpace(resp.ReadAllString())

	fmt.Println("Response from discovered provider:", text)
	if !strings.Contains(text, "Hello World!") {
		fmt.Fprintf(os.Stderr, "unexpected body: %q\n", text)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory to this consumer/ directory so it loads its
// own conf/app.properties (consumer/conf/app.properties) regardless of the
// process launch path. The provider does the same with its own conf, so the two
// no longer share a file.
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
