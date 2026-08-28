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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"go-spring.org/spring/gs"

	// Blank imports register the gateway and the OTel observability starters.
	// Routes are pure config (see conf/app.properties): the gateway listens on
	// :9440, proxies /api/** to an in-process upstream on :19000, and emits
	// traces to the OTLP endpoint plus Prometheus metrics on :9090.
	_ "go-spring.org/starter-otel"
	_ "go-spring.org/starter-gateway"
)

// backendAddr is the in-process upstream the gateway forwards to.
const backendAddr = "127.0.0.1:19000"

func main() {
	// Start the upstream so the example works with no external service. A real
	// collector is still needed for trace export (see README).
	startBackend()
	gs.Run()
}

// startBackend runs a tiny HTTP upstream reporting the path it received.
func startBackend() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path=%s\n", r.URL.Path)
	})
	ln, err := net.Listen("tcp", backendAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "backend listen:", err)
		os.Exit(1)
	}
	go func() { _ = http.Serve(ln, mux) }()
}

// init pins the working directory to this source file's directory so relative
// config loading (conf/app.properties) does not depend on the launch path.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
}
