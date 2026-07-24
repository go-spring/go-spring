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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gogf/gf/v2/net/gtcp"
	"go-spring.org/spring/gs"

	goframetcp "go-spring.org/starter-goframe/tcp"
)

func init() {
	// Provide a ServiceRegister bean that attaches a line-echo handler onto the
	// raw *gtcp.Server via SetHandler. The starter depends only on this function
	// type, so the concrete handler is wired here.
	gs.Provide(func() goframetcp.ServiceRegister {
		return func(s *gtcp.Server) {
			s.SetHandler(func(conn *gtcp.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					line, err := reader.ReadBytes('\n')
					if len(line) > 0 {
						if _, werr := conn.Write(line); werr != nil {
							return
						}
					}
					if err != nil {
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
	conn, err := gtcp.NewConn("127.0.0.1:8003")
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial failed:", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := conn.Send([]byte("ping\n")); err != nil {
		fmt.Fprintln(os.Stderr, "send failed:", err)
		os.Exit(1)
	}
	line, err := conn.RecvLine()
	if err != nil {
		fmt.Fprintln(os.Stderr, "recv failed:", err)
		os.Exit(1)
	}
	msg := strings.TrimSpace(string(line))
	fmt.Println("Response from server:", msg)

	if msg != "ping" {
		fmt.Fprintln(os.Stderr, "unexpected echo:", msg)
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
