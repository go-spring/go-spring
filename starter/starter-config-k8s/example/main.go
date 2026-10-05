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

// This example demonstrates the Kubernetes ConfigMap configuration provider and
// the ConfigMap -> bean hot-reload link:
//
//  1. app.properties imports a ConfigMap through the "k8s" provider via
//     spring.config.import=optional:k8s:configmap/app-config?...
//  2. A bean binds demo.message to a gs.Dync[string] field.
//  3. In a cluster the ConfigMap is read at startup and watched by an informer,
//     so an edit triggers a property refresh and the bound field updates
//     without a restart. Outside a cluster the import is "optional:", so the
//     read is skipped, the field shows its default, and the example
//     self-terminates cleanly.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"

	// Blank-import registers the "k8s" config provider, consumable via
	// spring.config.import with live hot-reload.
	_ "go-spring.org/starter-config-k8s"
)

// Demo binds a dynamic configuration field sourced from the imported ConfigMap.
// It is registered as a root object so the container creates it eagerly.
type Demo struct {
	Message gs.Dync[string] `value:"${demo.message:=none}"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	demoBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
	demo := demoBean.Interface().(*Demo)
	// Print on every change so a manual run (-manual) can watch the hot-reload
	// happen; the self-test reads the field directly.
	demo.Message.OnChanged(func(newVal, oldVal string) {
		if newVal != oldVal {
			fmt.Printf("demo.message: %q -> %q\n", oldVal, newVal)
		}
	})

	if !*manual {
		go func() {
			time.Sleep(500 * time.Millisecond)
			runTest(demo)
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// runTest reads the bound value and self-terminates. Outside a cluster the value
// is the default ("none"); in a cluster it is whatever the ConfigMap holds, and
// a subsequent `kubectl edit configmap app-config` would hot-reload it.
func runTest(d *Demo) {
	ctx := context.Background()
	fmt.Println("demo.message =", d.Message.Value())
	log.Info(ctx, log.TagAppDef,
		log.String("demo.message", d.Message.Value()),
		log.Msg("config-k8s example wired"))
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
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
