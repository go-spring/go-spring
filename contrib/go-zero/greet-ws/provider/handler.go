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
	"net/http"

	"github.com/zeromicro/go-zero/rest"
	"go-spring.org/spring/gs"

	"greetws/internal/handler"
	"greetws/internal/logic"
	"greetws/internal/svc"
)

func init() {
	// Provide a HandlerRegister bean that wires the injected GreetLogic bean
	// into a ServiceContext and attaches the WebSocket route. The route table
	// is hand-written here because there is no goctl-generated routes.go for
	// WS — the .api DSL cannot express a WS endpoint, so nothing is generated
	// (see scripts/gen-code.sh, which is a documented no-op for this reason).
	//
	// The route is a normal GET route; the handler upgrades to WS on demand.
	// go-zero's rest.Server response wrapper implements http.Hijacker, which
	// is what gorilla/websocket needs to steal the underlying TCP connection
	// away from the HTTP stack.
	gs.Provide(func(l *logic.GreetLogic) HandlerRegister {
		return func(server *rest.Server) {
			svcCtx := &svc.ServiceContext{Logic: l}
			server.AddRoutes([]rest.Route{
				{
					Method:  http.MethodGet,
					Path:    "/greet",
					Handler: handler.GreetWSHandler(svcCtx),
				},
			})
		}
	})
}
