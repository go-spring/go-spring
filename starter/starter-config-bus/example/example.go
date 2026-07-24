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

// This example demonstrates the config refresh bus:
//
//  1. app.properties defines a NATS instance named "config-bus" and binds
//     demo.message to a gs.Dync[string] field (initial value "v0").
//  2. The demo raises a new value from an environment source
//     (GS_DEMO_MESSAGE=v1) — a config change the local app would not notice on
//     its own — then publishes a single refresh event on the bus.
//  3. The bus subscriber (this same process, standing in for any instance in
//     the fleet) receives the event and re-runs the application property
//     refresh; the bound field flips to "v1" without a restart.
//
// A real deployment runs many instances sharing the subject: publishing once
// refreshes them all. Here one process both publishes and subscribes, which is
// enough to exercise the bus end to end.
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

	StarterConfigBus "go-spring.org/starter-config-bus"
	_ "go-spring.org/starter-nats"
)

// Demo binds a dynamic field and holds the bus so it can broadcast a refresh.
// It is registered as a root object so the container creates it eagerly.
type Demo struct {
	Bus     *StarterConfigBus.ConfigBus `autowire:"configBus"`
	Message gs.Dync[string]             `value:"${demo.message:=none}"`
}

func main() {
	demoBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(demoBean.Interface().(*Demo))
	}()

	gs.Run()
}

func runTest(d *Demo) {
	ctx := context.Background()

	// Raise a new value on an environment source the running app has not read
	// yet, then broadcast a single refresh so every subscriber re-reads it.
	want := "v-" + time.Now().Format("150405")
	_ = os.Setenv("GS_DEMO_MESSAGE", want)
	if err := d.Bus.Publish(""); err != nil {
		log.Errorf(ctx, log.TagAppDef, "publish refresh failed: %v", err)
		os.Exit(1)
	}

	// The bus subscriber triggers a property refresh, which re-reads sources and
	// updates the bound gs.Dync field. Poll until visible or time out.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if got := d.Message.Value(); got == want {
			fmt.Println("config-bus refresh observed:", got)
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Errorf(ctx, log.TagAppDef, "refresh timeout: message=%q want=%q", d.Message.Value(), want)
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
