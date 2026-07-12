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

	"go-spring.org/goframe/tcp/internal/server"
)

func init() {
	// The goframe *gtcp.Server, exported as a gs.Server so the Go-Spring
	// lifecycle starts and stops it. Config is bound from the
	// "${goframe.tcp}" prefix.
	//
	// Unlike ../http and ../grpc, gtcp has no built-in gsvc integration —
	// the adapter in internal/server/server.go performs Register /
	// Deregister by hand around the listener lifetime. This is the point
	// of this subproject: it shows how a non-HTTP goframe transport plugs
	// into the same etcd registry the sibling protocols use.
	gs.Provide(server.NewGoFrameTCPServer, gs.IndexArg(0, gs.TagArg("${goframe.tcp}"))).
		Export(gs.As[gs.Server]())
}

// The provider is a long-lived process: gs.Run() starts the GoFrameTCPServer
// registered above, which publishes itself into etcd, then blocks until it
// receives SIGTERM/SIGINT. Discovery and the TCP dial live in the consumer.
func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	// The built-in HTTP server is disabled via conf/app.properties; gs.Run()
	// starts only the GoFrameTCPServer registered above.
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
