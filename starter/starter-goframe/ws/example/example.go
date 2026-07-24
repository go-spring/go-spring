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
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gorilla/websocket"
	"go-spring.org/spring/gs"

	goframews "go-spring.org/starter-goframe/ws"
)

func init() {
	// Provide a ServiceRegister bean that binds the /echo route onto the raw
	// *ghttp.Server. The handler upgrades the connection to WebSocket and echoes
	// frames back — the only transport difference from the http sibling.
	gs.Provide(func() goframews.ServiceRegister {
		return func(s *ghttp.Server) {
			s.BindHandler("/echo", func(r *ghttp.Request) {
				ws, err := r.WebSocket()
				if err != nil {
					return
				}
				defer ws.Close()
				for {
					msgType, data, err := ws.ReadMessage()
					if err != nil {
						return
					}
					if err := ws.WriteMessage(msgType, data); err != nil {
						return
					}
				}
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
	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:8002/echo", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial failed:", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		fmt.Fprintln(os.Stderr, "write failed:", err)
		os.Exit(1)
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		fmt.Fprintln(os.Stderr, "read failed:", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", string(data))

	if string(data) != "ping" {
		fmt.Fprintln(os.Stderr, "unexpected echo:", string(data))
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
