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
	"github.com/gogf/gf/v2/net/ghttp"
	goframews "go-spring.org/starter-goframe/ws"
	"go-spring.org/spring/gs"
)

func init() {
	// Provide the starter's ServiceRegister bean that binds the /echo route onto
	// the raw *ghttp.Server. Importing the starter package (goframews) triggers
	// its module init, which registers the *ghttp.Server as a gs.Server; this
	// bean is the only wiring the application supplies — the server lifecycle and
	// log bridge live in the starter now (they used to be the deleted
	// provider/server.go).
	gs.Provide(func() goframews.ServiceRegister {
		return func(s *ghttp.Server) {
			s.BindHandler("/echo", echoHandler)
		}
	})
}

// echoHandler is the /echo route bound via the ServiceRegister bean above. It
// is a plain ghttp handler that promotes the connection to WebSocket. This is
// the *entire* transport difference vs the http sibling: instead of writing a
// response body, it calls r.WebSocket() to swap the HTTP connection for a
// gorilla-backed WebSocket, then runs a read/echo loop on it. Any HTTP-level
// middleware, timeouts and gsvc registration continue to apply to the initial
// handshake request.
func echoHandler(r *ghttp.Request) {
	ws, err := r.WebSocket()
	if err != nil {
		// Handshake failed (bad headers, wrong method, etc). ghttp has
		// already written a 4xx; nothing more to do.
		return
	}
	defer ws.Close()
	for {
		msgType, data, err := ws.ReadMessage()
		if err != nil {
			// Client closed the connection or the network broke; end the
			// echo loop. ws.Close() runs via defer.
			return
		}
		// Echo the frame back verbatim — same message type, same payload —
		// giving the consumer a deterministic value to assert on.
		if err := ws.WriteMessage(msgType, data); err != nil {
			return
		}
	}
}
