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
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"go-spring.org/spring/gs"

	greet "greetws/idl"
)

// Consumer holds the client-side settings injected from
// consumer/conf/app.properties. Unlike the sibling greet-rpc there is no etcd
// hop: WS rides on go-zero's rest.Server, which has no built-in service
// discovery, so the consumer takes the provider's WS endpoint from config and
// self-asserts on the echo frame.
type Consumer struct {
	Endpoint string `value:"${gozero.consumer.endpoint:=ws://127.0.0.1:8890/greet}"`
}

func main() {
	// Consumer is not referenced by any other bean, so register it as a root
	// object and grab the handle to drive the one-shot call from runTest.
	bean := gs.Provide(&Consumer{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(bean.Interface().(*Consumer))
	}()

	// The consumer runs server-less: no HTTP server (disabled in
	// consumer/conf/app.properties) and no rest.Server, so gs.Run() simply
	// blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest opens a persistent WS connection, exchanges a single frame, and
// closes cleanly. This is the piece that differs from greet-api's consumer:
// instead of one http.Post + JSON decode, we speak WebSocket. On success it
// sends SIGTERM so gs.Run() shuts down cleanly, making the process exit code
// the smoke-test result for scripts/smoke-test.sh.
func runTest(c *Consumer) {
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second

	conn, resp, err := dialer.Dial(c.Endpoint, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error dialing %s: %v\n", c.Endpoint, err)
		if resp != nil {
			fmt.Fprintf(os.Stderr, "handshake status: %s\n", resp.Status)
		}
		os.Exit(1)
	}
	defer conn.Close()

	req, err := json.Marshal(greet.GreetReq{Name: "Hello, go-zero!"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshalling request: %v\n", err)
		os.Exit(1)
	}
	if err := conn.WriteMessage(websocket.TextMessage, req); err != nil {
		fmt.Fprintf(os.Stderr, "error writing WS frame: %v\n", err)
		os.Exit(1)
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading WS frame: %v\n", err)
		os.Exit(1)
	}

	var out greet.GreetResp
	if err := json.Unmarshal(data, &out); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding response: %v\n", err)
		os.Exit(1)
	}

	// Close the WS connection cleanly so the provider's read loop returns
	// with CloseNormalClosure instead of a spurious error.
	_ = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	fmt.Println("Response from provider:", out.Greeting)
	if out.Greeting != "Hello, go-zero!" {
		fmt.Fprintf(os.Stderr, "unexpected greet body: %q\n", out.Greeting)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory to this consumer/ directory so it loads its
// own conf/app.properties (consumer/conf/app.properties) regardless of the
// process launch path. The provider does the same with its own conf, so the two
// no longer share a file.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	dir := filepath.Dir(filename)
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
