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
	greet "go-spring.org/dubbo-go/dubbo/proto"
	"go-spring.org/spring/gs"
	StarterDubbo "go-spring.org/starter-dubbo"
)

func init() {
	// Provide a StarterDubbo.ServiceRegister bean that binds the GreetProvider
	// to the Dubbo server. starter-dubbo's SimpleDubboServer depends only on this
	// function type, so the concrete service is wired here without the server
	// ever knowing about the greet interface.
	//
	// server.WithInterface(...) is what makes this the classic Dubbo protocol
	// rather than a bare RPC exposure: the provider registers into etcd under
	// the Java-style dotted interface name so cross-language consumers (Java
	// Dubbo included) can dial it by the same name.
	gs.Provide(func() StarterDubbo.ServiceRegister {
		return func(svr *server.Server) error {
			return svr.Register(&GreetProvider{}, nil,
				server.WithInterface(greet.GreetServiceInterface))
		}
	})
}

// GreetProvider implements the classic-Dubbo GreetService. The method
// signature is what gets wired: Dubbo-go reflects over exported methods and
// uses the method name (Greet) as the RPC procedure. Parameters and returns
// are marshalled with Hessian2, so plain Go primitives require no additional
// POJO registration.
type GreetProvider struct{}

// Greet echoes the request name back, giving the consumer a deterministic
// value to assert on.
func (s *GreetProvider) Greet(ctx context.Context, name string) (string, error) {
	return name, nil
}
