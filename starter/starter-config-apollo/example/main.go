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

// Package main is the example for starter-config-apollo. It starts a mock
// Apollo config service (apollo.go), imports the starter, and verifies the whole
// remote-config link: the property cold-loads into a Dync field at startup,
// then a publish advances the mock's notification id, agollo's long poll
// returns, and the starter triggers a refresh that updates the bound field —
// no docker, no real Apollo stack.
package main

import (
	"context"
	"flag"
	"fmt"
	"go-spring.org/stdlib/errutil"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-config-apollo"
)

// Demo holds a Dync property loaded from Apollo.
type Demo struct {
	Message gs.Dync[string] `value:"${demo.message:=none}"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling starter examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go mockConfigService()

	svrBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())
	demo := svrBean.Interface().(*Demo)
	// Print on every change so a manual run (-manual) can watch the hot-reload
	// happen; the self-test reads the field directly.
	demo.Message.OnChanged(func(newVal, oldVal string) {
		if newVal != oldVal {
			fmt.Printf("demo.message: %q -> %q\n", oldVal, newVal)
		}
	})

	if !*manual {
		go func() {
			time.Sleep(1 * time.Second)
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
		err := errutil.Explain(nil, "config mismatch: got %q, want %q", got, "hello-from-apollo")
		log.Error(ctx, log.TagAppDef, err,
			log.String("got", got),
			log.String("want", "hello-from-apollo"),
			log.Msg("CONFIG mismatch"))
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
	err := errutil.Explain(nil, "hot-reload timeout: got %q, want %q", d.Message.Value(), want)
	log.Error(ctx, log.TagAppDef, err,
		log.String("got", d.Message.Value()),
		log.String("want", want),
		log.Msg("hot-reload timeout"))
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
