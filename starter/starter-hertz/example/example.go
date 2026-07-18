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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go-spring.org/spring/gs"

	StarterHertz "go-spring.org/starter-hertz"
)

func init() {
	gs.Provide(&Controller{})
	// Provide a RouterRegister: the starter owns the *server.Hertz and its
	// listener (address from ${spring.hertz.server}); we only wire routes and
	// middleware onto the engine it hands us.
	gs.Provide(func(c *Controller) StarterHertz.RouterRegister {
		return func(h *server.Hertz) {
			// Feature 1: middleware — inject a response header on every request.
			h.Use(func(ctx context.Context, r *app.RequestContext) {
				r.Response.Header.Set("X-App", "go-spring")
				r.Next(ctx)
			})

			// Feature 2: path parameter + JSON response.
			h.GET("/echo/:name", c.Echo)

			// Feature 3: query parameter + JSON response.
			h.GET("/greet", c.Greet)
		}
	})
}

// Controller groups all HTTP handlers used by this example.
type Controller struct{}

// Echo answers with a JSON greeting built from the path parameter "name".
func (c *Controller) Echo(ctx context.Context, r *app.RequestContext) {
	name := r.Param("name")
	r.JSON(consts.StatusOK, map[string]string{"message": "Hello, " + name})
}

// Greet answers with a JSON greeting built from the query parameter "name".
func (c *Controller) Greet(ctx context.Context, r *app.RequestContext) {
	name := r.Query("name")
	r.JSON(consts.StatusOK, map[string]string{"message": "Hi, " + name})
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	// Configuration lives under ./conf/app.properties; the built-in HTTP
	// server is disabled there because Hertz drives its own on :8003.
	gs.Run()
}

func runTest() {
	// Feature 2: path parameter + JSON response.
	resp, err := http.Get("http://127.0.0.1:8003/echo/hertz")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request /echo/hertz failed:", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println("Response from server:", string(body))

	// Feature 1: middleware injected header.
	if got := resp.Header.Get("X-App"); got != "go-spring" {
		fmt.Fprintln(os.Stderr, "unexpected X-App header:", got)
		os.Exit(1)
	}

	var echoMsg map[string]string
	if err := json.Unmarshal(body, &echoMsg); err != nil {
		fmt.Fprintln(os.Stderr, "decode /echo/hertz failed:", err)
		os.Exit(1)
	}
	if echoMsg["message"] != "Hello, hertz" {
		fmt.Fprintln(os.Stderr, "unexpected /echo/hertz message:", echoMsg["message"])
		os.Exit(1)
	}

	// Feature 3: query parameter + JSON response.
	resp2, err := http.Get("http://127.0.0.1:8003/greet?name=world")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request /greet failed:", err)
		os.Exit(1)
	}
	body2, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	fmt.Println("Response from server:", string(body2))

	var greetMsg map[string]string
	if err := json.Unmarshal(body2, &greetMsg); err != nil {
		fmt.Fprintln(os.Stderr, "decode /greet failed:", err)
		os.Exit(1)
	}
	if greetMsg["message"] != "Hi, world" {
		fmt.Fprintln(os.Stderr, "unexpected /greet message:", greetMsg["message"])
		os.Exit(1)
	}

	// Server hardening: the starter-owned health endpoint responds independently
	// of any application route.
	hresp, err := http.Get("http://127.0.0.1:8003/healthz")
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
