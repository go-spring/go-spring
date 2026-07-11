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
	"strings"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/gsvc"
)

// The consumer never learns the provider's host:port. It builds a gclient
// bound to the same etcd registry and asks for the HTTP endpoint by the
// service name the provider registered under (goframe.hello by default);
// goframe's discovery middleware resolves a live provider address from etcd,
// dials it, and we assert on the response body.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	svcName := flag.String("service", "goframe.hello", "service name registered by the provider")
	flag.Parse()

	// Setting the global registry gives gclient a Discovery implementation via
	// gsvc.GetRegistry(); we still hand it to the client explicitly below,
	// which is closer to how registry wiring reads in the dubbo-go consumer.
	registry := etcdreg.New(*registryAddr)
	gsvc.SetRegistry(registry)

	ctx := context.Background()

	// Discovery(registry) turns http://<svcName>/<path> into an etcd lookup;
	// r.URL.Host in gclient's discovery middleware is treated as a service
	// name, not a network host.
	url := fmt.Sprintf("http://%s/hello", *svcName)
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
}
