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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-pprof"
)

func main() {
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling starter examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage (the pprof starter serves its endpoints on :9981 by default):
	//
	// ~ curl http://127.0.0.1:9981/debug/pprof/
	// ~ curl http://127.0.0.1:9981/debug/pprof/heap
	// ~ curl http://127.0.0.1:9981/debug/pprof/cmdline
}

// runTest verifies the three pprof endpoints served by starter-pprof's dedicated
// HTTP server, then triggers a graceful shutdown. Exits non-zero on any failure.
func runTest() {
	const base = "http://127.0.0.1:9981"

	endpoints := []string{
		"/debug/pprof/",
		"/debug/pprof/heap",
		"/debug/pprof/cmdline",
	}

	for _, path := range endpoints {
		resp, err := http.Get(base + path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "request failed:", path, err)
			os.Exit(1)
		}
		// Drain the body so the connection can be reused / closed cleanly.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintln(os.Stderr, "unexpected status:", path, resp.StatusCode)
			os.Exit(1)
		}
		fmt.Println("Response from server:", path, resp.Status)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

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
