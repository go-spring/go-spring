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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterSwagger "go-spring.org/starter-swagger"
)

func init() {
	// The swagger starter contributes the Swagger UI as an endpoint.Endpoint
	// (auto-mounted by the actuator) and as a concrete *StarterSwagger.UI. This
	// example does not run the actuator, so we mount the UI on the built-in gs
	// http server ourselves by providing a *gs.HttpServeMux that delegates the
	// UI's subtree to it.
	gs.Provide(func(ui *StarterSwagger.UI) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.Handle(ui.Path(), ui)
		return &gs.HttpServeMux{Handler: mux}
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
	ctx := context.Background()

	// Feature 1: the UI shell is served at the base path and references the
	// Swagger UI assets plus our own spec URL.
	page := mustGet(ctx, "http://127.0.0.1:9696/swagger/")
	if !strings.Contains(page, "swagger-ui") || !strings.Contains(page, "openapi.json") {
		log.Errorf(ctx, log.TagAppDef, "UI page missing expected markers: %q", page)
		os.Exit(1)
	}
	fmt.Println("Response from server: /swagger/ served the Swagger UI shell")

	// Feature 2: index.html resolves to the same shell.
	if idx := mustGet(ctx, "http://127.0.0.1:9696/swagger/index.html"); !strings.Contains(idx, "swagger-ui") {
		log.Errorf(ctx, log.TagAppDef, "index.html missing UI markers: %q", idx)
		os.Exit(1)
	}
	fmt.Println("Response from server: /swagger/index.html served the same shell")

	// Feature 3: the generated OpenAPI document is served verbatim.
	spec := mustGet(ctx, "http://127.0.0.1:9696/swagger/openapi.json")
	if !strings.Contains(spec, "\"openapi\"") || !strings.Contains(spec, "Greeter API") {
		log.Errorf(ctx, log.TagAppDef, "spec missing expected content: %q", spec)
		os.Exit(1)
	}
	fmt.Println("Response from server: /swagger/openapi.json served the OpenAPI document")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// mustGet fetches url and returns its body, exiting the process on any error or
// non-200 status so the smoke test fails loudly.
func mustGet(ctx context.Context, url string) string {
	resp, err := http.Get(url)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "GET %s failed: %v", url, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Errorf(ctx, log.TagAppDef, "GET %s status=%d body=%q", url, resp.StatusCode, string(body))
		os.Exit(1)
	}
	return string(body)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory where
// this source file resides, so relative file operations (loading openapi.json)
// resolve against the source location, not the process launch path.
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
