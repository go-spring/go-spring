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

	"go-spring.org/spring/gs"

	"greet/internal/server"
	"greet/internal/svc"
)

func init() {
	// ServiceContext carries shared dependencies into handlers/logic.
	// go-zero builds it in main(); here the container owns it, with the
	// config bound from properties under the "${greet}" prefix.
	gs.Provide(svc.NewServiceContext, gs.IndexArg(0, gs.TagArg("${greet}")))

	// The go-zero rest.Server, exported as a gs.Server so the Go-Spring
	// lifecycle starts and stops it. This replaces main()'s
	// rest.MustNewServer + server.Start() calls.
	gs.Provide(server.NewGreetServer, gs.IndexArg(0, gs.TagArg("${greet}"))).
		Export(gs.As[gs.Server]())
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

// runTest exercises the converted go-zero handler over HTTP, then asks the
// process to shut down so the example terminates on its own.
func runTest() {
	resp, err := http.Get("http://localhost:8888/from/you")
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println("Response from server:", string(body))

	var greetResp map[string]string
	if err := json.Unmarshal(body, &greetResp); err != nil {
		fmt.Fprintln(os.Stderr, "invalid JSON from /from/:name:", err)
		os.Exit(1)
	}
	if greetResp["message"] != "Hello, you" {
		fmt.Fprintln(os.Stderr, "unexpected message:", greetResp["message"])
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory to the directory of this source file so that
// conf/app.properties resolves regardless of where the process is launched.
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
