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

	"github.com/labstack/echo/v4"
	"go-spring.org/spring/gs"

	StarterEcho "go-spring.org/starter-echo"
)

func init() {
	gs.Provide(&Controller{})
	// Provide a RouterRegister: the starter owns the *echo.Echo and its HTTP
	// server (address from ${spring.echo.server}); we only wire routes and
	// middleware onto the engine it hands us.
	gs.Provide(func(c *Controller) StarterEcho.RouterRegister {
		return func(e *echo.Echo) {
			// Feature 1: middleware — stamp an application-level header on
			// every response so callers can identify the service.
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(ctx echo.Context) error {
					ctx.Response().Header().Set("X-App", "go-spring")
					return next(ctx)
				}
			})

			// Feature 2: path parameter + JSON response.
			e.GET("/echo/:name", c.Echo)

			// Feature 3: route group with a query-parameter handler.
			g := e.Group("/api")
			g.GET("/greet", c.Greet)
		}
	})
}

type Controller struct{}

// Echo returns a JSON greeting using the `:name` path parameter.
func (c *Controller) Echo(ctx echo.Context) error {
	name := ctx.Param("name")
	return ctx.JSON(http.StatusOK, map[string]string{
		"message": "Hello, " + name,
	})
}

// Greet returns a JSON greeting using the `name` query parameter.
func (c *Controller) Greet(ctx echo.Context) error {
	name := ctx.QueryParam("name")
	return ctx.JSON(http.StatusOK, map[string]string{
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
	// Feature 1: custom middleware sets the X-App header.
	resp, err := http.Get("http://localhost:8002/echo/echo")
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

	// Feature 2: path parameter -> JSON body.
	var echoResp map[string]string
	if err := json.Unmarshal(body, &echoResp); err != nil {
		fmt.Fprintln(os.Stderr, "invalid JSON from /echo/:name:", err)
		os.Exit(1)
	}
	if echoResp["message"] != "Hello, echo" {
		fmt.Fprintln(os.Stderr, "unexpected /echo/:name message:", echoResp["message"])
		os.Exit(1)
	}

	// Feature 3: route group + query parameter.
	resp, err = http.Get("http://localhost:8002/api/greet?name=world")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println("Response from server:", string(body))
	var greetResp map[string]string
	if err := json.Unmarshal(body, &greetResp); err != nil {
		fmt.Fprintln(os.Stderr, "invalid JSON from /api/greet:", err)
		os.Exit(1)
	}
	if greetResp["message"] != "Hi, world" {
		fmt.Fprintln(os.Stderr, "unexpected /api/greet message:", greetResp["message"])
		os.Exit(1)
	}

	// Server hardening: the starter-owned health endpoint responds independently
	// of any application route.
	hresp, err := http.Get("http://localhost:8002/healthz")
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
