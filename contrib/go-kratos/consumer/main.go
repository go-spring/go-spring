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
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	transgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/gorilla/websocket"
	clientv3 "go.etcd.io/etcd/client/v3"
	v1 "go-spring.org/go-kratos/api/helloworld/v1"
)

// wsHelloMessageType MUST match provider/handler.go's WSHelloMessageType.
// Since the WebSocket transport is message-typed rather than RPC-typed, this
// integer is the contract by which the server routes a frame to a handler.
const wsHelloMessageType uint32 = 1

// The kratos-transport WebSocket server is configured with PayloadTypeBinary,
// so every frame on the wire is
//
//	<4-byte little-endian uint32 messageType><JSON-encoded payload bytes>
//
// (see BinaryNetPacket in kratos-transport/transport/websocket/message.go).
// This format is symmetric — the server sends replies in the same shape — so
// we can hand-craft one small marshal/unmarshal pair for the smoke test.

// The consumer never learns the provider's gRPC host:port. It builds an
// etcd-backed registry.Discovery and asks kratos to dial
// "discovery:///<name>" — the same service name the provider registered
// under. kratos resolves a live provider instance from etcd, dials it via
// gRPC, and we assert on the echo.
//
// After the gRPC round-trip we also exercise the WebSocket transport that
// the provider now hosts alongside HTTP+gRPC. WebSocket dialing is done
// directly against the configured ws:// URL rather than through kratos
// discovery: kratos-transport's WS client has no discovery hook, and adding
// one just to demo an extra transport would obscure what the transport does.
// The gRPC path already proves discovery works; WS proves coexistence.
func main() {
	registryAddr := flag.String("registry", "127.0.0.1:2379", "etcd registry address")
	svcName := flag.String("service", "kratos-greeter", "kratos service name to resolve")
	wsURL := flag.String("ws", "ws://127.0.0.1:9002/", "kratos WebSocket transport endpoint")
	flag.Parse()

	ctx := context.Background()

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{*registryAddr},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd client: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	r := etcd.New(cli)

	conn, err := transgrpc.DialInsecure(ctx,
		transgrpc.WithEndpoint("discovery:///"+*svcName),
		transgrpc.WithDiscovery(r),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial discovered provider: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	resp, err := v1.NewGreeterClient(conn).SayHello(ctx, &v1.HelloRequest{Name: "Kratos"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calling SayHello: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Response from discovered provider (gRPC):", resp.Message)
	if resp.Message != "Hello Kratos" {
		fmt.Fprintf(os.Stderr, "unexpected gRPC reply: %q\n", resp.Message)
		os.Exit(1)
	}

	// WebSocket leg — same provider, different transport, different wire.
	if err := roundTripWebSocket(*wsURL, "Kratos-WS"); err != nil {
		fmt.Fprintf(os.Stderr, "websocket check failed: %v\n", err)
		os.Exit(1)
	}
}

// roundTripWebSocket dials the kratos-transport WS server and performs a
// single request/response using the PayloadTypeBinary wire format. Any
// deviation (non-zero close, wrong message type, wrong echo body) fails the
// smoke test.
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

	// Build the binary frame: 4 little-endian bytes of the message type,
	// followed by the JSON-encoded payload.
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
