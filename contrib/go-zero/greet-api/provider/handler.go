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
	"github.com/zeromicro/go-zero/rest"
	"go-spring.org/spring/gs"

	"greetapi/handler"
	"greetapi/svc"
)

func init() {
	// Provide a HandlerRegister bean that wires the injected GreetLogic bean
	// into a ServiceContext and hands it to the goctl-generated route table.
	// The RestServer adapter (see server.go) depends only on this function
	// type, so the concrete route registration stays here without the server
	// ever knowing about it — mirroring the ServiceRegister pattern in the
	// sibling greet-rpc.
	gs.Provide(func(l *svc.GreetLogic) HandlerRegister {
		return func(server *rest.Server) {
			handler.RegisterHandlers(server, &svc.ServiceContext{Logic: l})
		}
	})
}
