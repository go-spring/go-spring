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

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	"go-spring.org/goframe/grpc/pbgen/echo"
	"go-spring.org/spring/gs"
)

// Consumer holds the client-side settings injected from
// consumer/conf/app.properties. It never learns the provider's host:port: it
// resolves a live provider from the same etcd registry the provider published
// into, by the service name the provider registered under.
type Consumer struct {
	RegistryAddr string `value:"${goframe.consumer.registry.etcd:=127.0.0.1:2379}"`
	ServiceName  string `value:"${goframe.consumer.service.name:=goframe.grpc.echo}"`
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
	// consumer/conf/app.properties) and no grpcx server, so gs.Run() simply
	// blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest registers the same etcd registry the provider used, then calls
// grpcx.Client.MustNewGrpcClientConn with the service name: grpcx builds a
// gsvc://<name> target, goframe's discovery resolver watches that name in etcd
// and returns a live endpoint to the underlying *grpc.ClientConn. This is the
// microservice governance path grpcx advertises, not a direct dial. On success
// it sends SIGTERM so gs.Run() shuts down cleanly.
func runTest(c *Consumer) {
	// Register the etcd registry with grpcx. This must go through
	// grpcx.Resolver.Register (not gsvc.SetRegistry): grpcx's init() already
	// registered a grpc resolver builder bound to the then-nil default
	// registry, so it also has to re-register the builder with a real
	// discovery — which Resolver.Register does and SetRegistry alone does not.
	// Passing the same address the provider registered against is what turns
	// the service name below into a live endpoint.
	grpcx.Resolver.Register(etcdreg.New(c.RegistryAddr))

	conn := grpcx.Client.MustNewGrpcClientConn(c.ServiceName)
	defer conn.Close()

	cli := echo.NewEchoServiceClient(conn)

	want := "Hello, GoFrame gRPC!"
	resp, err := cli.Echo(context.Background(), &echo.EchoRequest{Message: want})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling Echo: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider:", resp.Message)
	if resp.Message != want {
		fmt.Fprintf(os.Stderr, "unexpected echo body: %q\n", resp.Message)
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
