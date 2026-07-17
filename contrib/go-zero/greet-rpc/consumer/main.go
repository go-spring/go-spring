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

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"

	"go-spring.org/spring/gs"

	greet "greetrpc/idl"
)

// Consumer holds the client-side settings injected from
// consumer/conf/app.properties. It never learns the provider's host:port: it
// resolves a live provider from the same etcd registry the provider published
// into, by the etcd key the provider registered under.
type Consumer struct {
	RegistryAddr string `value:"${gozero.consumer.registry:=127.0.0.1:2379}"`
	EtcdKey      string `value:"${gozero.consumer.key:=greet.rpc}"`
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
	// consumer/conf/app.properties) and no zrpc server (no ServiceRegister
	// bean), so gs.Run() simply blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest builds a zrpc client bound to the etcd registry and asks for the
// "greet.rpc" key; zrpc resolves a live provider address from etcd, calls it,
// and we assert on the echo. On success it sends SIGTERM so gs.Run() shuts
// down cleanly, making the process exit code the smoke-test result for
// scripts/smoke-test.sh.
func runTest(c *Consumer) {
	ctx := context.Background()

	cli := zrpc.MustNewClient(zrpc.RpcClientConf{
		Etcd: discov.EtcdConf{
			Hosts: []string{c.RegistryAddr},
			Key:   c.EtcdKey,
		},
	})

	client := greet.NewGreetClient(cli.Conn())

	resp, err := client.Greet(ctx, &greet.GreetReq{Name: "Hello, go-zero!"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling Greet: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider:", resp.Greeting)
	if resp.Greeting != "Hello, go-zero!" {
		fmt.Fprintf(os.Stderr, "unexpected greet body: %q\n", resp.Greeting)
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
