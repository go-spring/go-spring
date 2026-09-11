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

// Command example wires starter-registry-etcd into a Go-Spring application:
// blank-importing the starter plus a ${spring.registry.etcd.main} block
// creates the backend bean "etcd.main"; ${spring.registry.service-name} then
// registers this instance into etcd once the app is ready and deregisters it
// on shutdown (through the starter-registry core), while the same bean serves
// as the consumer-side discovery backend cited by its name.
//
// To make the mechanism visible without an external client, a Runner resolves
// the just-registered instance back through the discovery backend, prints what
// it found, then SIGTERMs the app so the example self-terminates (exercising
// the deregister-on-shutdown path). It needs a reachable etcd at the
// configured address; check.sh starts one in Docker.
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

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	// Blank-import registers the center module: the shared client, the
	// register-on-ready server, and the derived discovery backend.
	_ "go-spring.org/starter-registry-etcd"
)

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	if !*manual {
		// The verify runner (registered in init below) resolves the instance
		// once registration has happened, then SIGTERMs the app so the example
		// terminates (and deregisters) on its own.
		gs.Provide(NewVerifyRunner).Export(gs.As[gs.Runner]())
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// VerifyRunner resolves this process's own registration back through the
// etcd discovery backend "etcd.main", proving the register→discover loop end to end.
type VerifyRunner struct {
	// backend is the backend bean named "etcd.main" — the named block it was
	// configured from.
	backend discovery.Discovery `autowire:"etcd.main"`
}

// NewVerifyRunner builds the verify runner.
func NewVerifyRunner() *VerifyRunner { return &VerifyRunner{} }

// Run spawns the verify loop in the background and returns immediately: a
// blocking Runner would delay the readiness signal the registry server waits
// for, deadlocking registration (servers register only once the app is ready,
// which requires all Runners to have returned).
func (v *VerifyRunner) Run(ctx context.Context) error {
	go v.verify(ctx)
	return nil
}

// verify polls the discovery backend until the registry server (which
// registers on app readiness) has published this instance, prints what it
// found, then stops the app so the example self-terminates.
func (v *VerifyRunner) verify(ctx context.Context) {
	d := v.backend
	if d == nil {
		log.Errorf(ctx, log.TagAppDef, "discovery backend %q not wired", "etcd.main")
		return
	}
	var eps []discovery.Endpoint
	var err error
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		eps, err = d.Resolve(ctx, "orders")
		if err == nil && len(eps) > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		log.Warnf(ctx, log.TagAppDef, "resolve orders failed: %v", err)
	} else if len(eps) == 0 {
		log.Warnf(ctx, log.TagAppDef, "service orders has no instances yet")
	}
	for _, e := range eps {
		fmt.Printf("discovered endpoint=%s weight=%d metadata=%v\n", e.Addr, e.Weight, e.Metadata)
	}
	// One-shot: stop the app so the example terminates (and deregisters).
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
