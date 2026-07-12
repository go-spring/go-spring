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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"go-spring.org/spring/gs"

	"go-spring.org/goframe/websocket/internal/server"
)

func init() {
	// The goframe *ghttp.Server, exported as a gs.Server so the Go-Spring
	// lifecycle starts and stops it. Config is bound from the
	// "${goframe.websocket}" prefix.
	//
	// WebSocket in goframe is not a separate server: the same *ghttp.Server
	// that would answer HTTP requests upgrades to WebSocket on the /echo
	// route (see internal/server/server.go). Registration into etcd via
	// gsvc happens at HTTP-server granularity, which is why the WS variant
	// still ends up using the exact same lifecycle bean as the http sibling.
	gs.Provide(server.NewGoFrameServer, gs.IndexArg(0, gs.TagArg("${goframe.websocket}"))).
		Export(gs.As[gs.Server]())
}

// The provider is a long-lived process: gs.Run() starts the GoFrameServer
// registered above, which publishes itself into etcd, then blocks until it
// receives SIGTERM/SIGINT. Discovery and the WebSocket dial live in the
// consumer.
func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	// The built-in HTTP server is disabled via conf/app.properties; gs.Run()
	// starts only the GoFrameServer registered above.
	gs.Run()
}

// init sets the working directory to the module root (the parent of this
// provider/ directory) so relative config loading (conf/app.properties) works
// regardless of the process launch path.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	moduleRoot := filepath.Dir(filepath.Dir(filename))
	if err := os.Chdir(moduleRoot); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
