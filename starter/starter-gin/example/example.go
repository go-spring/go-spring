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
	"encoding/json"
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
	gs.Provide(&Controller{})
	// Provide a RouterRegister: the starter owns the *gin.Engine and its HTTP
	// server (address from ${spring.gin.server}); we only wire routes and
	// middleware onto the engine it hands us.
	gs.Provide(func(c *Controller) StarterGin.RouterRegister {
		return func(e *gin.Engine) {
			// Feature 1: middleware — stamp an application-level header on
			// every response so callers can identify the service.
			e.Use(func(ctx *gin.Context) {
				ctx.Header("X-App", "go-spring")
				ctx.Next()
			})

			// Feature 2: path parameter + JSON response.
			e.GET("/echo/:name", c.Echo)

			// Feature 3: query-parameter handler.
			e.GET("/greet", c.Greet)
		}
	})
}

type Controller struct{}

// Echo returns a JSON greeting using the `:name` path parameter.
func (c *Controller) Echo(ctx *gin.Context) {
	name := ctx.Param("name")
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Hello, " + name,
	})
}

// Greet returns a JSON greeting using the `name` query parameter.
func (c *Controller) Greet(ctx *gin.Context) {
	name := ctx.Query("name")
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Hi, " + name,
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
	// Feature 1 + Feature 2: custom middleware sets the X-App header
	// and the path parameter is echoed back as JSON.
	resp, err := http.Get("http://127.0.0.1:8001/echo/gin")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println("Response from server:", string(body))
	if got := resp.Header.Get("X-App"); got != "go-spring" {
		fmt.Fprintln(os.Stderr, "unexpected X-App header:", got)
		os.Exit(1)
	}
	// Built-in middleware (default-on): every response carries a request id
	// generated/propagated by the RequestID middleware.
	if rid := resp.Header.Get("X-Request-Id"); rid == "" {
		fmt.Fprintln(os.Stderr, "missing X-Request-Id header")
		os.Exit(1)
	}
	// Opt-in middleware: SecureHeaders stamps safe response headers.
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		fmt.Fprintln(os.Stderr, "unexpected X-Content-Type-Options header:", got)
		os.Exit(1)
	}
	var echoResp map[string]string
	if err := json.Unmarshal(body, &echoResp); err != nil {
		fmt.Fprintln(os.Stderr, "invalid JSON from /echo/:name:", err)
		os.Exit(1)
	}
	if echoResp["message"] != "Hello, gin" {
		fmt.Fprintln(os.Stderr, "unexpected /echo/:name message:", echoResp["message"])
		os.Exit(1)
	}

	// Feature 3: query parameter handler.
	resp, err = http.Get("http://127.0.0.1:8001/greet?name=world")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println("Response from server:", string(body))
	var greetResp map[string]string
	if err := json.Unmarshal(body, &greetResp); err != nil {
		fmt.Fprintln(os.Stderr, "invalid JSON from /greet:", err)
		os.Exit(1)
	}
	if greetResp["message"] != "Hi, world" {
		fmt.Fprintln(os.Stderr, "unexpected /greet message:", greetResp["message"])
		os.Exit(1)
	}

	// Server hardening: the starter-owned health endpoint responds independently
	// of any application route.
	hresp, err := http.Get("http://127.0.0.1:8001/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, "health request failed:", err)
		os.Exit(1)
	}
	hbody, _ := io.ReadAll(hresp.Body)
	_ = hresp.Body.Close()
	fmt.Println("Health from server:", string(hbody))
	if hresp.StatusCode != http.StatusOK || string(hbody) != "ok" {
		fmt.Fprintln(os.Stderr, "unexpected /healthz response:", hresp.StatusCode, string(hbody))
		os.Exit(1)
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
