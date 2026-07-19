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

// Command gateway is the edge of the full-stack reference app. It runs
// starter-gateway on its own port and routes every /api/** request to the order
// service (A), discovered through Consul (lb://order) rather than a hard-coded
// address, so scaling or restarting A needs no gateway config change.
//
// Routing is pure config (see conf/app.properties); the only code here registers
// the Consul-backed discovery backend that the lb:// upstream resolves through.
// Authentication is deliberately NOT done here: A is the JWT resource server, and
// the gateway forwards the caller's Authorization header untouched, so the token
// is verified at the service that owns the protected resource.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"go-spring.org/spring/gs"

	"fullstack/internal/consuldisc"

	// The gateway server + route table, plus observability.
	_ "go-spring.org/starter-actuator"
	_ "go-spring.org/starter-gateway"
	_ "go-spring.org/starter-otel"
)

// discoveryName is the stdlib/discovery registry key the gateway's
// spring.gateway.routes.orders.upstream.discovery references.
const discoveryName = "consul"

// consulAddr is the Consul agent the gateway resolves lb:// upstreams against.
const consulAddr = "127.0.0.1:8500"

func init() {
	// Publish the Consul resolver before the gateway compiles its lb://order
	// upstream (route compilation is deferred to warmup/first use, so registering
	// here is in time). discovery.MustGet(discoveryName) inside the gateway then
	// finds it.
	if err := consuldisc.Register(discoveryName, consulAddr); err != nil {
		panic(err)
	}
}

func main() {
	gs.Run()
}

// init sets the working directory to this source file's directory so the
// relative conf/app.properties path resolves regardless of the launch path.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
	wd, _ := os.Getwd()
	fmt.Println("gateway workdir:", wd)
}
