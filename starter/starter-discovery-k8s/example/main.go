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

// Command example demonstrates wiring starter-discovery-k8s into a Go-Spring
// application. Blank-importing the starter and declaring one
// "${spring.discovery.k8s.<name>}" entry registers a discovery backend under
// that name; any client (Redis/GORM/...) then resolves a Kubernetes Service by
// setting its `discovery:` field to the same name.
//
// This program has no external client — it resolves a target Service directly
// through discovery.MustGet to make the mechanism visible, prints the live
// endpoints, then exits. Run inside a cluster (see deploy/) it prints the
// target Deployment's ready Pods; run locally without cluster DNS it prints the
// resolve error and still exits cleanly, since the point is to show the API.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/gs"

	// Blank-import registers the Kubernetes discovery backend(s) declared under
	// spring.discovery.k8s.
	_ "go-spring.org/starter-discovery-k8s"
)

// backendName matches the map key in conf/app.properties
// (spring.discovery.k8s.<backendName>); targetService is the Kubernetes
// Service name to resolve through it.
const (
	backendName   = "k8s"
	targetService = "demo"
)

func main() {
	go func() {
		// Give the container a moment to register the backend during refresh.
		time.Sleep(500 * time.Millisecond)
		resolveOnce()
		// One-shot: stop the app so the example terminates on its own.
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()

	gs.Run()
}

// resolveOnce looks up the target Service through the registered backend and
// logs the outcome. It never calls os.Exit(1) on a resolve error: outside a
// cluster the lookup is expected to fail, and the example's job is to show the
// call, not to assert on a cluster that may be absent.
func resolveOnce() {
	ctx := context.Background()
	d, err := discovery.MustGet(backendName)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "discovery backend %q not registered: %v", backendName, err)
		return
	}
	eps, err := d.Resolve(ctx, targetService)
	if err != nil {
		log.Warnf(ctx, log.TagAppDef, "resolve %q failed (expected outside a cluster): %v", targetService, err)
		return
	}
	if len(eps) == 0 {
		log.Warnf(ctx, log.TagAppDef, "service %q resolved to no endpoints", targetService)
		return
	}
	for _, ep := range eps {
		fmt.Printf("endpoint addr=%s healthy=%v zone=%s\n", ep.Addr, ep.Healthy, ep.Metadata["zone"])
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
