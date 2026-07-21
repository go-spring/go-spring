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

	kws "github.com/tx7do/kratos-transport/transport/websocket"
	v1 "go-spring.org/go-kratos-ws/idl/helloworld/v1"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	kratosws "go-spring.org/starter-kratos/ws"
)

// WSHelloMessageType is the application-defined message-type discriminator
// carried in every WebSocket frame's envelope. Unlike proto RPCs, kratos-
// transport WebSocket is a raw framed pipe: server and client MUST agree on
// this integer out of band. Keeping the constant here (mirrored by the
// consumer) is the simplest form of that contract.
const WSHelloMessageType kws.NetMessageType = 1

// WSHelloRequest and WSHelloReply are the WS-side payload shapes. They are
// intentionally NOT the protoc-generated v1.HelloRequest/HelloReply: those
// types carry proto-internal fields that leak into JSON output and would force
// the consumer to depend on the proto package just to hand-craft a text frame.
// Duplicating a two-field struct keeps the WS contract self-contained.
type WSHelloRequest struct {
	Name string `json:"name"`
}

type WSHelloReply struct {
	Message string `json:"message"`
}

func init() {
	// Provide the single-argument ServiceRegister bean starter-kratos/ws depends
	// on: it binds WSHelloMessageType to a handler on the kratos-transport WS
	// server. The starter's WsServer only materializes because this bean exists.
	gs.Provide(func() kratosws.ServiceRegister {
		greeter := &GreeterService{}
		return func(ws *kws.Server) error {
			// WebSocket is message-typed, not RPC-typed. Bind WSHelloMessageType
			// to a handler that reuses GreeterService.SayHello and echoes the
			// reply asynchronously via ws.SendMessage — WS is full-duplex, so
			// unlike request/response transports we don't "return" a value.
			kws.RegisterServerMessageHandler(ws, WSHelloMessageType,
				func(sessionID kws.SessionID, req *WSHelloRequest) error {
					reply, err := greeter.SayHello(context.Background(),
						&v1.HelloRequest{Name: req.Name})
					if err != nil {
						return err
					}
					return ws.SendMessage(sessionID, WSHelloMessageType,
						&WSHelloReply{Message: reply.Message})
				})
			return nil
		}
	})
}

// GreeterService implements the greeting logic, folded directly into the
// handler (matching the flat provider/handler.go shape the other contrib
// examples use).
type GreeterService struct {
	v1.UnimplementedGreeterServer
}

// SayHello echoes the request name back as "Hello <name>", giving the consumer
// a deterministic value to assert on over WebSocket.
func (s *GreeterService) SayHello(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	log.Infof(ctx, log.TagBizDef, "SayHello name=%s", in.Name)
	return &v1.HelloReply{Message: "Hello " + in.Name}, nil
}
