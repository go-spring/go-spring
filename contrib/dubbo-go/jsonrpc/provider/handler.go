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

	"dubbo.apache.org/dubbo-go/v3/server"
	greet "go-spring.org/dubbo-go/jsonrpc/proto"
	"go-spring.org/spring/gs"
	StarterDubbo "go-spring.org/starter-dubbo"
)

func init() {
	// Provide a StarterDubbo.ServiceRegister bean that binds the GreetProvider
	// to the Dubbo server. starter-dubbo's DubboServer depends only on this
	// function type, so the concrete service is wired here without the server
	// ever knowing about the greet interface.
	//
	// server.WithInterface(...) publishes the provider into etcd under the
	// Java-style dotted interface name; the JSON-RPC HTTP path served on the
	// wire is /<interface-name>, and the JSON-RPC "method" field carries the
	// method name — so cross-language consumers can hand-craft an HTTP POST
	// and invoke the service directly.
	gs.Provide(func() StarterDubbo.ServiceRegister {
		return func(svr *server.Server) error {
			return svr.Register(&GreetProvider{}, nil,
				server.WithInterface(greet.GreetServiceInterface))
		}
	})
}

// GreetProvider implements the JSON-RPC GreetService. As with the classic-Dubbo
// sibling, dubbo-go reflects over exported methods and uses the method name
// (Greet) as the RPC procedure. Parameters and returns are marshalled with
// encoding/json, so any JSON-serializable type works out of the box; there
// is no equivalent to Hessian2's POJO registration table.
type GreetProvider struct{}

// Greet echoes the request name back, giving the consumer a deterministic
// value to assert on.
func (s *GreetProvider) Greet(ctx context.Context, name string) (string, error) {
	return name, nil
}
