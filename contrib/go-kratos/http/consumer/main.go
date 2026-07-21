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

	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "go-spring.org/go-kratos-http/idl/helloworld/v1"
	"go-spring.org/spring/gs"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Consumer holds the client-side settings injected from
// consumer/conf/app.properties. It never learns the provider's host:port: it
// resolves a live provider from the same etcd registry the provider registered
// into, by the service name below.
type Consumer struct {
	RegistryAddr string `value:"${kratos.consumer.registry.etcd:=127.0.0.1:2379}"`
	ServiceName  string `value:"${kratos.consumer.service.name:=kratos-http}"`
}

// The consumer never learns the provider's HTTP host:port. It builds an
// etcd-backed registry.Discovery and asks the kratos HTTP client to dial
// "discovery:///<name>" — the same service name the provider registered under.
// kratos resolves a live provider instance from etcd, dials it over HTTP, and
// we assert on the echo.
func main() {
	// Consumer is not referenced by any other bean, so register it as a root
	// object and grab the handle to drive the one-shot call from runTest.
	bean := gs.Provide(&Consumer{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(bean.Interface().(*Consumer))
	}()

	// The consumer runs server-less: no HTTP server (disabled in
	// consumer/conf/app.properties) and no kratos server (no ServiceRegister
	// bean), so gs.Run() simply blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest performs the HTTP discovery call. On success it sends SIGTERM so
// gs.Run() shuts down cleanly, making the process exit code the smoke-test
// result for scripts/smoke-test.sh.
func runTest(c *Consumer) {
	ctx := context.Background()

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{c.RegistryAddr},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd client: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	r := etcd.New(cli)

	conn, err := khttp.NewClient(ctx,
		khttp.WithEndpoint("discovery:///"+c.ServiceName),
		khttp.WithDiscovery(r),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial discovered provider: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	resp, err := v1.NewGreeterHTTPClient(conn).SayHello(ctx, &v1.HelloRequest{Name: "Kratos"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling SayHello: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider (HTTP):", resp.Message)
	if resp.Message != "Hello Kratos" {
		fmt.Fprintf(os.Stderr, "unexpected HTTP reply: %q\n", resp.Message)
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
