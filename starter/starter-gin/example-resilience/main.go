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

// Command example demonstrates starter-gin's inbound resilience inbound: with
// spring.gin.server.resilience.enabled, every request runs through the selected
// resilience driver's chain.Executor, so a burst over the configured rate limit is
// shed with HTTP 429 (circuit-open with 503) before the business handler runs.
//
// This is entirely config-driven — the starter applies the inbound middleware
// itself; the app only wires a route. The smoke test fires a burst and asserts
// the server both serves and sheds load. No external services or docker are
// required.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go-spring.org/spring/gs"

	StarterGin "go-spring.org/starter-gin"
)

func init() {
	gs.Provide(func() StarterGin.RouterRegister {
		return func(e *gin.Engine) {
			e.GET("/", func(ctx *gin.Context) {
				ctx.JSON(http.StatusOK, gin.H{"message": "hello"})
			})
		}
	})
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest()
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// runTest fires a burst at the rate-limited route and asserts that inbound
// both serves (200) and sheds (429) load. Exits non-zero on failure.
func runTest() {
	const url = "http://127.0.0.1:8081/"
	var ok, limited int
	for range 20 {
		resp, err := http.Get(url)
		if err != nil {
			fail("request: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			ok++
		case http.StatusTooManyRequests:
			limited++
		default:
			fail("unexpected status %d", resp.StatusCode)
		}
	}
	if ok == 0 || limited == 0 {
		fail("resilience inbound ineffective: ok=%d limited=%d", ok, limited)
	}
	fmt.Printf("gin resilience: %d served, %d rejected with 429\n", ok, limited)

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}

// init sets the working directory to this source file's directory so relative
// config lookups (conf/) resolve against the source location.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
