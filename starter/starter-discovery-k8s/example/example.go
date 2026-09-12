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
// "${spring.discovery.k8s.instances.<name>}" entry registers a discovery backend under
// that name; any client (Redis/GORM/...) then resolves a Kubernetes Service by
// setting its `discovery:` field to the same name.
//
// This program has no external client — it resolves a target Service directly
// through the injected backend bean to make the mechanism visible, prints the live
// endpoints, then exits. Run inside a cluster (see deploy/) it prints the
// target Deployment's ready Pods; run locally without cluster DNS it prints the
// resolve error and still exits cleanly, since the point is to show the API.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	// Blank-import registers the Kubernetes discovery backend(s) declared under
	// spring.discovery.k8s.instances.
	_ "go-spring.org/starter-discovery-k8s"
)

// backendName matches the map key in conf/app.properties
// (spring.discovery.k8s.instances.<backendName>); targetService is the Kubernetes
// Service name to resolve through it.
const (
	backendName   = "k8s"
	targetService = "demo"
)

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	if *manual {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	} else {
		// One-shot mode: the resolveRunner below resolves the target Service
		// once the app is wired, then stops the app so the example terminates
		// on its own.
		gs.Provide(newResolveRunner).Export(gs.As[gs.Runner]())
	}
	gs.Run()
}

// resolveRunner resolves the target Service once through the injected backend
// and then stops the app (one-shot mode). It never os.Exit(1)s on a resolve
// error: outside a cluster the lookup is expected to fail, and the example's
// job is to show the call, not to assert on a cluster that may be absent.
type resolveRunner struct {
	// backend is the discovery bean named after the spring.discovery.k8s.instances.<name>
	// map key in conf/app.properties.
	backend discovery.Discovery `autowire:"k8s"`
}

// newResolveRunner builds the one-shot resolve runner.
func newResolveRunner() *resolveRunner { return &resolveRunner{} }

// Run resolves the target Service in the background (a blocking Runner would
// delay readiness) and stops the app when it has logged the outcome.
func (r *resolveRunner) Run(ctx context.Context) error {
	go func() {
		defer func() { _ = syscall.Kill(os.Getpid(), syscall.SIGTERM) }()
		d := r.backend
		if d == nil {
			log.Errorf(ctx, log.TagAppDef, "discovery backend %q not wired", backendName)
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
	}()
	return nil
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
