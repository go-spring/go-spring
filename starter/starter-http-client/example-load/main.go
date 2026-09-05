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

// Package main is the load-test binary for starter-http-client. It spins up an
// in-process HTTP backend (no docker), wires a declarative http-client against
// it (resilience + fault hot-reloadable), and drives GETs through the shared
// [loadtest] harness. Toggle fault.* in conf/app.properties to "set fire".
package main

import (
	"context"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/cloud/loadtest"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/httpclt"

	_ "go-spring.org/starter-http-client"
)

var (
	concurrency = flag.Int("concurrency", 8, "concurrent workers")
	duration    = flag.Duration("duration", 5*time.Second, "load duration")
	manual      = flag.Bool("manual", false, "keep the process running for ad-hoc probing")
)

const backendAddr = "127.0.0.1:18080"

func main() {
	flag.Parse()

	// In-process backend the declarative client calls. Dedicated ServeMux so it
	// never clashes with the gs built-in server.
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("pong")) })
	srv := &http.Server{Addr: backendAddr, Handler: mux}
	go func() { _ = srv.ListenAndServe() }()

	if !*manual {
		go func() {
			time.Sleep(500 * time.Millisecond) // let gs wire + the client build
			runLoad()
		}()
	} else {
		println("=== Manual mode: load runs once on startup; Ctrl+C to stop ===")
	}
	gs.Run()
}

func runLoad() {
	ctx := context.Background()
	url := "http://" + backendAddr + "/ping"
	op := func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Host = backendAddr // dispatch routes by req.Host
		_, err = httpclt.DoRequest(req, httpclt.Metadata{}, func(r io.Reader) error {
			_, _ = io.Copy(io.Discard, r)
			return nil
		})
		return err
	}
	loadtest.Run(ctx, loadtest.Config{Concurrency: *concurrency, Duration: *duration}, op).Print(os.Stdout)
	if !*manual {
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}
}

// init pins the working directory to this file's directory so conf/ resolves.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		if err := os.Chdir(filepath.Dir(filename)); err != nil {
			panic(err)
		}
	}
}
