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

// Command example wires starter-registry-zookeeper into a Go-Spring
// application: blank-importing the starter plus a
// ${spring.registry.zookeeper.servers} entry registers this instance into
// ZooKeeper once the app is ready and deregisters it on shutdown.
//
// To make the mechanism visible without an external client, a goroutine waits
// for readiness, lists the registered znodes back, prints what it finds, then
// SIGTERMs the app so the example self-terminates (exercising the
// deregister-on-shutdown path). It needs a reachable ZooKeeper at the
// configured address; check.sh starts one in Docker.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/go-zookeeper/zk"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	// Blank-import registers the ZooKeeper registrar and the register-on-ready server.
	_ "go-spring.org/starter-registry-zookeeper"
)

// servicePath matches ${spring.registry.zookeeper.base-path} + service name.
const servicePath = "/services/orders"

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

// verifyOnce lists the registered znodes and logs the instances found. It never
// exits non-zero on failure: the point is to show the call; check.sh asserts the
// end-to-end path separately.
func verifyOnce() {
	ctx := context.Background()
	conn, _, err := zk.Connect([]string{"127.0.0.1:2181"}, 10*time.Second)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "zookeeper connect: %v", err)
		return
	}
	defer conn.Close()

	children, _, err := conn.Children(servicePath)
	if err != nil {
		log.Warnf(ctx, log.TagAppDef, "list %q failed (is ZooKeeper reachable?): %v", servicePath, err)
		return
	}
	if len(children) == 0 {
		log.Warnf(ctx, log.TagAppDef, "path %q has no instances yet", servicePath)
		return
	}
	for _, child := range children {
		data, _, err := conn.Get(servicePath + "/" + child)
		if err != nil {
			continue
		}
		fmt.Printf("registered node=%s value=%s\n", child, data)
	}
}

// init sets the working directory to this source file's directory so
// conf/app.properties loads regardless of where the process is launched.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
}
