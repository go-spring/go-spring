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

	greet "go-spring.org/dubbo-go/proto"
	"go-spring.org/spring/gs"
)

func init() {
	// Register the provider as a greet.GreetServiceHandler bean. The Dubbo
	// server adapter (see server.go) declares a dependency on this interface,
	// so exporting it here is what wires the generated service into Go-Spring's
	// IoC container instead of the scaffold's hand-built main().
	gs.Provide(&GreetProvider{}).Export(gs.As[greet.GreetServiceHandler]())
}

// GreetProvider implements the GreetServiceHandler interface generated from
// greet.proto.
type GreetProvider struct{}

// Greet echoes the request name back as the greeting, giving the client a
// deterministic value to assert on.
func (s *GreetProvider) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
	return &greet.GreetResponse{Greeting: req.Name}, nil
}
