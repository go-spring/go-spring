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
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"

	greet "greetws/proto"
)

// The consumer dials the provider's WebSocket endpoint directly. Unlike the
// sibling greet-rpc there is no etcd hop: WS rides on go-zero's rest.Server,
// which has no built-in service discovery, so we take the provider's
// host:port on the command line and self-assert on the echo frame.
//
// This is the piece that differs from greet-api's consumer: instead of one
// http.Post + JSON decode, we open a persistent WS connection, exchange a
// single frame, and close cleanly. Everything else — flag, exit-code
// contract, assertion — matches the sibling.
func main() {
	endpoint := flag.String("endpoint", "ws://127.0.0.1:8890/greet", "provider WS endpoint")
	flag.Parse()

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second

	conn, resp, err := dialer.Dial(*endpoint, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error dialing %s: %v\n", *endpoint, err)
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
}
