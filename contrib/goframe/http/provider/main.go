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

	_ "go-spring.org/starter-otel"
	"go-spring.org/spring/gs"
)

// The provider is a long-lived process: gs.Run() starts the goframe *ghttp.Server
// contributed by starter-goframe/http (registered as a gs.Server when the
// application provides a ServiceRegister bean — see provider/handler.go), which
// publishes itself into etcd, then blocks until it receives SIGTERM/SIGINT.
// Unlike the old single-process example there is no inline client here —
// discovery and the HTTP call live in the consumer.
//
// starter-otel is imported for its side effect: it installs the global
// OpenTelemetry TracerProvider from ${spring.observability}, and goframe's ghttp
// auto-instruments every request off that global provider. This replaces the
// inline contrib/trace/otlphttp wiring the deleted provider/server.go carried.
func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	// The built-in HTTP server is disabled via provider/conf/app.properties;
	// gs.Run() starts only the goframe server contributed by the starter.
	gs.Run()
}

// init sets the working directory to this provider/ directory so it loads its
// own conf/app.properties (provider/conf/app.properties) regardless of the
// process launch path. The consumer does the same with its own conf, so the two
// no longer share a file.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	dir := filepath.Dir(filename)
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
