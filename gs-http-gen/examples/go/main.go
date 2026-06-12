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
	"time"

	"examples/ginsvr"
	"examples/proto"
	"examples/server"

	"github.com/go-spring/stdlib/httpsvr"
)

// init sets the working directory of the program to the directory
// where this source file resides. This ensures that relative paths
// used later in the program (e.g., for output) are resolved correctly.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine caller directory")
	}
	execDir := filepath.Dir(filename)
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println("working directory:", workDir)
}

func main() {
	TestManager()
	TestStream()
}

func TestManager() {
	svr := ginsvr.NewGinServer(":9191")
	for _, r := range proto.Routers(&server.ManagerServer{}, ginsvr.NewGinRequestContext) {
		svr.HandleFunc(r)
	}
	go func() {
		fmt.Println(svr.ListenAndServe())
	}()
	time.Sleep(time.Millisecond * 300)

	resp, err := http.Get("http://localhost:9191/managers/123")
	if err != nil {
		panic(err)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	fmt.Println(string(b))
	svr.Shutdown(context.Background())
}

func TestStream() {
	svr := httpsvr.NewSimpleServer(":9191")
	for _, r := range proto.Routers(&server.ManagerServer{}, httpsvr.NewSimpleContext) {
		svr.Route(r)
	}
	go func() {
		fmt.Println(svr.ListenAndServe())
	}()
	time.Sleep(time.Millisecond * 300)

	resp, err := http.PostForm("http://localhost:9191/assistant/stream", nil)
	if err != nil {
		panic(err)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1025))
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	fmt.Print(string(b))
	svr.Shutdown(context.Background())
}
