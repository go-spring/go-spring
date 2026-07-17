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

	"github.com/zeromicro/go-zero/rest"
	"go-spring.org/spring/gs"

	gozerorest "go-spring.org/starter-go-zero/rest"
)

func init() {
	// Provide a HandlerRegister bean: this is the only thing the application
	// supplies. The starter's RestServer picks it up (via OnBean[HandlerRegister])
	// and hands it the *rest.Server to attach routes onto — the server itself
	// stays service-agnostic.
	gs.Provide(func() gozerorest.HandlerRegister {
		return func(server *rest.Server) {
			server.AddRoute(rest.Route{
				Method: http.MethodGet,
				Path:   "/greet",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					name := r.URL.Query().Get("name")
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]string{
						"message": "Hi, " + name,
					})
				},
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
	resp, err := http.Get("http://127.0.0.1:8888/greet?name=world")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
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

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory to this source file's directory so the process
// loads its own conf/app.properties regardless of the launch path.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
