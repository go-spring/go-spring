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

// Command example wires starter-registry-consul into a Go-Spring application:
// blank-importing the starter plus a ${spring.registry.consul.address} entry
// registers this instance into Consul once the app is ready and deregisters it
// on shutdown.
//
// To make the mechanism visible without an external client, a goroutine waits
// for readiness, queries the Consul catalog for the just-registered service,
// prints what it finds, then SIGTERMs the app so the example self-terminates
// (exercising the deregister-on-shutdown path). It needs a reachable Consul
// agent at the configured address; check.sh starts one in Docker.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	// Blank-import registers the Consul registrar and the register-on-ready server.
	_ "go-spring.org/starter-registry-consul"
)

// serviceName matches ${spring.registry.service-name} in conf/app.properties.
const serviceName = "orders"

func main() {
	go func() {
		// Wait past readiness so registration has happened, then verify.
		time.Sleep(time.Second)
		verifyOnce()
		// One-shot: stop the app so the example terminates (and deregisters) on its own.
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()

	gs.Run()
}

// verifyOnce reads the Consul catalog for the registered service and logs the
// instances found. It never exits non-zero on failure: the point is to show the
// call; check.sh asserts the end-to-end path separately.
func verifyOnce() {
	ctx := context.Background()
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:8500"})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "consul client: %v", err)
		return
	}
	services, _, err := client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		log.Warnf(ctx, log.TagAppDef, "query %q failed (is Consul reachable?): %v", serviceName, err)
		return
	}
	if len(services) == 0 {
		log.Warnf(ctx, log.TagAppDef, "service %q has no healthy instances yet", serviceName)
		return
	}
	for _, s := range services {
		fmt.Printf("registered addr=%s:%d meta=%v\n", s.Service.Address, s.Service.Port, s.Service.Meta)
	}
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
