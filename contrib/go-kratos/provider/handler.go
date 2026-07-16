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

	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	kws "github.com/tx7do/kratos-transport/transport/websocket"
	v1 "go-spring.org/go-kratos/api/helloworld/v1"
	"go-spring.org/spring/gs"
)

// WSHelloMessageType is the application-defined message-type discriminator
// carried in every WebSocket frame's envelope (`{"type":<N>,...}`). Unlike
// HTTP+gRPC, whose routing is derived from the proto RPC name, kratos-transport
// WebSocket is a raw framed pipe: server and client MUST agree on this integer
// out of band. Keeping the constant here (shared with the consumer) is the
// simplest form of that contract.
const WSHelloMessageType kws.NetMessageType = 1

// WSHelloRequest and WSHelloReply are the WS-side payload shapes. They are
// intentionally NOT the protoc-generated v1.HelloRequest/HelloReply: those
// types carry proto-internal fields (`state`, `sizeCache`, `unknownFields`)
// that leak into JSON output and would force the consumer to depend on the
// proto package just to hand-craft a text frame. Duplicating a two-field
// struct keeps the WS contract self-contained and debuggable via wscat.
type WSHelloRequest struct {
	Name string `json:"name"`
}

type WSHelloReply struct {
	Message string `json:"message"`
}

func init() {
	// Provide a ServiceRegister bean that binds the GreeterService to all three
	// kratos transport servers. KratosServer (see server.go) depends only on
	// this function type, so the concrete service is wired here without the
	// adapter ever knowing about v1.GreeterServer or WS message types.
	gs.Provide(func() ServiceRegister {
		greeter := &GreeterService{}
		return func(hs *khttp.Server, gs *kgrpc.Server, ws *kws.Server) error {
			// HTTP + gRPC share the proto contract: one Register call per
			// transport, both dispatch to the same GreeterService.
			v1.RegisterGreeterHTTPServer(hs, greeter)
			v1.RegisterGreeterServer(gs, greeter)

			// WebSocket is message-typed, not RPC-typed. Bind
			// WSHelloMessageType -> a handler that reuses the same
			// GreeterService.SayHello as the HTTP+gRPC transports (so all three
			// serve the same business logic) and echoes the reply asynchronously
			// via ws.SendMessage — WS is full-duplex, so unlike request/response
			// transports we don't "return" a value from the handler.
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

// GreeterService implements the GreeterServer interface generated from
// helloworld.proto. It replaces the kratos scaffold's internal/{biz,service,data}
// layering, which for this example was three empty passthroughs (the data repo
// returned the greeter unchanged and the biz usecase only logged). Folding the
// greeting directly into the handler matches the flat provider/handler.go shape
// the dubbo-go examples use.
type GreeterService struct {
	v1.UnimplementedGreeterServer
}

// SayHello echoes the request name back as "Hello <name>", giving the consumer
// a deterministic value to assert on over HTTP, gRPC and WebSocket.
func (s *GreeterService) SayHello(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	return &v1.HelloReply{Message: "Hello " + in.Name}, nil
}
