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
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"go-spring.org/spring/gs"

	greet "greetws/idl"
)

func init() {
	// Register GreetLogic as an IoC bean and a HandlerRegister bean that
	// attaches the WebSocket route, invoking the injected logic per frame.
	//
	// There is no goctl-generated routes.go for WS — the .api DSL cannot
	// express a WS endpoint, so nothing is generated (see idl/gen-code.sh,
	// a documented no-op) and, unlike the sibling greet-api, there is no
	// separate handler/ package. Everything the WS endpoint needs lives here
	// in the provider, mirroring how the dubbo-go examples keep handler +
	// logic together under provider/.
	//
	// The route is a normal GET route; the handler upgrades to WS on demand.
	// go-zero's rest.Server response wrapper implements http.Hijacker, which
	// is what gorilla/websocket needs to steal the underlying TCP connection
	// away from the HTTP stack.
	gs.Provide(&GreetLogic{})
	gs.Provide(func(l *GreetLogic) HandlerRegister {
		return func(server *rest.Server) {
			server.AddRoutes([]rest.Route{
				{
					Method:  http.MethodGet,
					Path:    "/greet",
					Handler: greetWSHandler(l),
				},
			})
		}
	})
}

// GreetLogic implements the Greet operation. It is stateless in this example
// but stays a struct so it can hold injected dependencies later without
// touching the handler / route wiring. In a stock go-zero project this would
// be scaffolded under internal/logic; here it is a Go-Spring bean so the same
// stateless logic could just as easily serve HTTP, zRPC or WS.
type GreetLogic struct{}

// Greet echoes the request name back as the greeting so the consumer has a
// deterministic value to assert on. Called once per WS frame; the handler
// keeps invoking it in a loop for the lifetime of the connection.
func (l *GreetLogic) Greet(ctx context.Context, req *greet.GreetReq) (*greet.GreetResp, error) {
	return &greet.GreetResp{Greeting: req.Name}, nil
}

// upgrader turns an inbound HTTP request into a WebSocket connection. The
// zero-value CheckOrigin rejects cross-origin browser clients by default;
// this example allows all origins because the consumer is a local process,
// not a browser, and adding an allow-list would only obscure the flow.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// greetWSHandler returns an http.HandlerFunc that upgrades the request to a
// WebSocket connection and, for each JSON frame it receives, invokes the
// injected GreetLogic bean and writes the response frame back.
//
// This is the piece that differs from the sibling greet-api handler:
//   - The handler owns the connection for its whole lifetime instead of
//     returning after a single request/response.
//   - httpx.Parse / httpx.OkJsonCtx are irrelevant once the connection is
//     upgraded; framing goes through websocket.Conn.
//   - The upgrade only works if the underlying http.ResponseWriter supports
//     http.Hijacker. go-zero's rest.Server response wrapper does support
//     hijacking, so we can add a WS route the same way we would any REST
//     route.
func greetWSHandler(logic *GreetLogic) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Upgrade already wrote the HTTP error response; just log.
			logx.WithContext(r.Context()).Errorf("ws upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		ctx := r.Context()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				// A normal client-side Close arrives as CloseNormalClosure
				// or a *net.OpError; either way there is nothing to log.
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway) {
					logx.WithContext(ctx).Errorf("ws read: %v", err)
				}
				return
			}
			if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
				continue
			}

			var req greet.GreetReq
			if err := json.Unmarshal(data, &req); err != nil {
				_ = conn.WriteMessage(websocket.TextMessage,
					[]byte(`{"error":"bad json"}`))
				continue
			}

			resp, err := logic.Greet(ctx, &req)
			if err != nil {
				_ = conn.WriteMessage(websocket.TextMessage,
					[]byte(`{"error":"logic failed"}`))
				continue
			}

			out, err := json.Marshal(resp)
			if err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, out); err != nil {
				logx.WithContext(ctx).Errorf("ws write: %v", err)
				return
			}
		}
	}
}
