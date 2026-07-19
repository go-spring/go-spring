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

// Command example demonstrates wiring starter-config-k8s: importing a
// ConfigMap through the "k8s" provider and binding one of its keys to a
// hot-reloadable gs.Dync field.
//
// The import in conf/app.properties is marked "optional:", so the app boots
// cleanly whether or not a cluster is reachable:
//
//   - Outside a cluster (local `go run .`): the API read is skipped, the bound
//     field shows its default, and the example self-terminates — proving the
//     wiring compiles and registers without a control plane.
//   - In a cluster (see deploy/): the ConfigMap is read at startup and an
//     informer watches it, so `kubectl edit configmap app-config` updates the
//     bound field within seconds, no restart and no volume mount.
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
	"go-spring.org/spring/gs"

	// Blank-import registers the "k8s" config provider consumed via
	// spring.app.imports.
	_ "go-spring.org/starter-config-k8s"
)

// Demo binds a dynamic configuration field sourced from the imported ConfigMap.
// It is registered as a root object so the container creates it eagerly.
type Demo struct {
	Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func main() {
	// Unset shell-leaked env vars so runs are reproducible across examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	demoBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		report(demoBean.Interface().(*Demo))
	}()

	gs.Run()
}

// report prints the bound value and self-terminates. Outside a cluster the value
// is the default ("none"); in a cluster it is whatever the ConfigMap holds, and
// a subsequent `kubectl edit configmap app-config` would hot-reload it.
func report(d *Demo) {
	ctx := context.Background()
	fmt.Println("demo.message =", d.Message.Value())
	log.Infof(ctx, log.TagAppDef, "config-k8s example wired; demo.message=%q", d.Message.Value())
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
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
