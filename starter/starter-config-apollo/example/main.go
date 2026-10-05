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

// This example demonstrates the Apollo remote configuration provider and the
// namespace -> bean hot-reload link:
//
//  1. app.properties imports config from a mock Apollo service started by this
//     example via spring.config.import=optional:apollo:.../application?appId=demo
//     (no docker, no real Apollo stack).
//  2. A bean binds demo.message to a gs.Dync[string] field.
//  3. The example publishes a new value to the mock; agollo's long poll returns
//     on the notification id bump, the starter triggers a property refresh, and
//     the bound field updates without a restart.
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
	"go-spring.org/stdlib/errutil"

	// Blank-import registers the "apollo" config provider, consumable via
	// spring.config.import with live hot-reload.
	_ "go-spring.org/starter-config-apollo"
)

// Demo binds a dynamic configuration field sourced from the imported Apollo
// namespace. It is registered as a root object so the container creates it
// eagerly.
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

	mockConfigService()

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

func runTest(d *Demo) {
	ctx := context.Background()

	// Cold load: the imported namespace was read at startup.
	got := d.Message.Value()
	if got != "hello-from-apollo" {
		err := errutil.Explain(nil, "got %q, want %q", got, "hello-from-apollo")
		log.Errorf(ctx, log.TagAppDef, err, "config mismatch")
		os.Exit(1)
	}
	fmt.Println("Apollo cold-load OK:", got)

	// Publish a new value: the mock bumps its notification id, agollo's long
	// poll returns and it re-fetches the namespace, and the starter triggers a
	// property refresh that updates the bound gs.Dync field. Poll until the new
	// value is visible or time out.
	want := "hello-" + time.Now().Format("150405")
	store.set(want)

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if d.Message.Value() == want {
			fmt.Println("hot-reload observed:", want)
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	err := errutil.Explain(nil, "got %q, want %q", d.Message.Value(), want)
	log.Errorf(ctx, log.TagAppDef, err, "hot-reload timeout")
	os.Exit(1)
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
