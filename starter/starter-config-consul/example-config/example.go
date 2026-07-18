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

// This example demonstrates the Consul remote configuration provider and the
// KV -> bean hot-reload link:
//
//  1. app.properties imports config from Consul via
//     spring.app.imports=optional:consul:.../gs-config-demo?...
//  2. A bean binds demo.message to a gs.Dync[string] field.
//  3. The example publishes a new value to Consul; the provider's blocking
//     query wakes up on the index bump, triggers a property refresh, and the
//     bound field updates without a restart.
//
// The publisher client below is built directly from the SDK rather than
// injected, keeping the demonstration focused on the provider and refresh
// link.
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

	_ "go-spring.org/starter-config-consul"
)

const (
	kvKey       = "gs-config-demo"
	consulAddr  = "127.0.0.1:8500"
)

// Demo binds a dynamic configuration field sourced from the imported Consul KV
// path. It is registered as a root object so the container creates it eagerly.
type Demo struct {
	Message gs.Dync[string] `value:"${demo.message:=none}"`
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

	// Publish a new value for the imported KV path via the Consul HTTP API.
	want := "hello-" + time.Now().Format("150405")
	if err := publish(want); err != nil {
		log.Errorf(ctx, log.TagAppDef, "publish config failed: %v", err)
		os.Exit(1)
	}

	// The provider's watcher observes the KV index bump and triggers a
	// property refresh, which re-fetches the config and updates the bound
	// gs.Dync field. Poll until the new value is visible or time out.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if got := d.Message.Value(); got == want {
			fmt.Println("hot-reload observed:", got)
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Errorf(ctx, log.TagAppDef, "hot-reload timeout: message=%q want=%q", d.Message.Value(), want)
	os.Exit(1)
}

// publish writes demo.message=<value> to the Consul KV path.
func publish(value string) error {
	cli, err := api.NewClient(&api.Config{Address: consulAddr, Scheme: "http"})
	if err != nil {
		return err
	}
	_, err = cli.KV().Put(&api.KVPair{
		Key:   kvKey,
		Value: []byte("demo.message=" + value),
	}, nil)
	return err
}

// init sets the working directory to this source file's directory so relative
// config paths resolve correctly.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine source file path")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
}
