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

	greet "go-spring.org/dubbo-go/triple/idl"
	StarterDubbo "go-spring.org/starter-dubbo"
)

func init() {
	// Register the GreetProvider as a Dubbo service via starter-dubbo's helper.
	// RegisterService binds ${spring.dubbo.server.services.greet} (per-service
	// overrides, including per-method tuning under .methods) and turns it into
	// dubbo-go server.ServiceOption passed to the generated handler - so the
	// service is exported with that config without the server knowing about
	// greet.GreetServiceHandler. T is inferred from the generated handler's
	// parameter type (the handler interface); the concrete &GreetProvider{} is
	// converted to that interface at the call site (a Go generics limitation).
	StarterDubbo.RegisterService("greet", greet.RegisterGreetServiceHandler,
		greet.GreetServiceHandler(&GreetProvider{}))
}

// GreetProvider implements the GreetServiceHandler interface generated from
// greet.proto.
type GreetProvider struct{}

// Greet echoes the request name back as the greeting, giving the consumer a
// deterministic value to assert on.
func (s *GreetProvider) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
	return &greet.GreetResponse{Greeting: req.Name}, nil
}
