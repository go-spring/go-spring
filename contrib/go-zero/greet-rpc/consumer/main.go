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
	"flag"
	"fmt"
	"os"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"

	greet "greetrpc/proto"
)

// The consumer never learns the provider's host:port. It builds a zrpc client
// bound to the same etcd registry and asks for the "greet.rpc" key; zrpc
// resolves a live provider address from etcd, calls it, and we assert on the
// echo.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	etcdKey := flag.String("key", "greet.rpc", "etcd key the provider registered under")
	flag.Parse()

	ctx := context.Background()

	cli := zrpc.MustNewClient(zrpc.RpcClientConf{
		Etcd: discov.EtcdConf{
			Hosts: []string{*registryAddr},
			Key:   *etcdKey,
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
}
