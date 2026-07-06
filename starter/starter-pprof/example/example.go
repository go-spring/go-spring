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
	"net/http"
	"os"
	"syscall"
	"time"

	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-pprof"
)

func main() {
	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()
	gs.Run()
}

// runTest verifies the pprof endpoint is served, then triggers shutdown.
func runTest() {
	resp, err := http.Get("http://127.0.0.1:9090/debug/pprof/")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unexpected status:", resp.StatusCode)
		os.Exit(1)
	}
	fmt.Println("Response from server: pprof")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// browser: http://127.0.0.1:9090/debug/pprof/
