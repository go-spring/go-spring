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
	greet "go-spring.org/dubbo-go/dubbo/idl"
	StarterDubbo "go-spring.org/starter-dubbo"
)

func init() {
	// Register the GreetProvider via starter-dubbo's helper. RegisterService
	// binds ${spring.dubbo.server.services.greet} (per-service overrides +
	// per-method tuning) into the register bean and turns it into dubbo-go
	// server.ServiceOption, threaded here into the reflective svr.Register
	// alongside the essential WithInterface. T is the concrete *GreetProvider
	// (classic Dubbo has no generated handler, so a closure wraps svr.Register).
	// server.WithInterface is what makes this classic Dubbo rather than a bare
	// RPC exposure: the provider registers into etcd under the Java-style dotted
	// interface name so cross-language consumers (Java Dubbo included) can dial
	// it by the same name.
	StarterDubbo.RegisterService("greet",
		func(svr *server.Server, hdlr *GreetProvider, opts ...server.ServiceOption) error {
			opts = append([]server.ServiceOption{server.WithInterface(greet.GreetServiceInterface)}, opts...)
			return svr.Register(hdlr, nil, opts...)
		}, &GreetProvider{})
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
