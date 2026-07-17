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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"go-spring.org/spring/gs"
)

// wsHelloMessageType mirrors WSHelloMessageType from the provider: kratos-
// transport WebSocket carries no proto-encoded RPCs, just <4-byte
// messageType><JSON payload> frames, so consumer and provider agree on the
// integer discriminator out of band.
const wsHelloMessageType uint32 = 1

// Consumer holds the client-side settings injected from
// consumer/conf/app.properties. Unlike the http/grpc consumers this one never
// touches etcd: kratos-transport's WebSocket client has no discovery hook, so
// the consumer dials the provider's WS endpoint directly.
type Consumer struct {
	WSURL string `value:"${kratos.consumer.ws.url:=ws://127.0.0.1:9002/}"`
}

// The consumer dials the kratos-transport WebSocket server directly and runs a
// single request/response round trip using the PayloadTypeBinary framing the
// server speaks. On success it sends SIGTERM so gs.Run() shuts down cleanly.
func main() {
	// Consumer is not referenced by any other bean, so register it as a root
	// object and grab the handle to drive the one-shot call from runTest.
	bean := gs.Provide(&Consumer{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(bean.Interface().(*Consumer))
	}()

	// The consumer runs server-less: no HTTP server (disabled in
	// consumer/conf/app.properties) and no kratos server (no ServiceRegister
	// bean), so gs.Run() simply blocks until runTest sends SIGTERM.
	gs.Run()
}

// runTest performs the WebSocket round trip. On success it sends SIGTERM so
// gs.Run() shuts down cleanly, making the process exit code the smoke-test
// result for scripts/smoke-test.sh.
func runTest(c *Consumer) {
	if err := roundTripWebSocket(c.WSURL, "Kratos-WS"); err != nil {
		fmt.Fprintf(os.Stderr, "ws round trip failed: %v\n", err)
		os.Exit(1)
	}
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// roundTripWebSocket dials the kratos-transport WS server and performs a single
// request/response using the PayloadTypeBinary wire format: every frame is
// <4-byte little-endian uint32 messageType><JSON payload>. Any deviation fails.
func roundTripWebSocket(endpoint, name string) error {
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", endpoint, err)
	}
	defer conn.Close()

	payload, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return fmt.Errorf("marshal ws payload: %w", err)
	}
	frame := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint32(frame[:4], wsHelloMessageType)
	copy(frame[4:], payload)
	if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		return fmt.Errorf("write ws message: %w", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read ws message: %w", err)
	}
	if len(data) < 4 {
		return fmt.Errorf("ws reply too short: %d bytes", len(data))
	}
	replyType := binary.LittleEndian.Uint32(data[:4])
	if replyType != wsHelloMessageType {
		return fmt.Errorf("unexpected ws message type: got %d, want %d", replyType, wsHelloMessageType)
	}
	var reply struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data[4:], &reply); err != nil {
		return fmt.Errorf("unmarshal ws reply: %w", err)
	}
	want := "Hello " + name
	fmt.Println("Response from discovered provider (WebSocket):", reply.Message)
	if reply.Message != want {
		return fmt.Errorf("unexpected ws reply: got %q, want %q", reply.Message, want)
	}
	return nil
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
