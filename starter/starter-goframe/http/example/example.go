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
	"strings"
	"syscall"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"go-spring.org/spring/gs"

	goframehttp "go-spring.org/starter-goframe/http"
)

func init() {
	// Provide a ServiceRegister bean: the only thing the application supplies.
	// The starter's HTTPServer picks it up (via OnBean[ServiceRegister]) and
	// hands it the response-wrapping router group to attach routes onto.
	gs.Provide(func() goframehttp.ServiceRegister {
		return func(group *ghttp.RouterGroup) {
			// Write directly to the response buffer so goframe's
			// MiddlewareHandlerResponse leaves the body untouched (an existing
			// buffer is not re-wrapped in the JSON envelope).
			group.ALL("/hello", func(r *ghttp.Request) {
				r.Response.Writeln("Hello World!")
			})
		}
	})
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	gs.Run()
}

func runTest() {
	resp, err := http.Get("http://127.0.0.1:8000/hello")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println("Response from server:", string(body))

	if !strings.Contains(string(body), "Hello World!") {
		fmt.Fprintln(os.Stderr, "unexpected /hello body:", string(body))
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
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
