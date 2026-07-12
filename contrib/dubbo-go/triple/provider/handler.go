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
	greet "go-spring.org/dubbo-go/triple/proto"
	"go-spring.org/spring/gs"
)

func init() {
	// Provide a ServiceRegister bean that binds the GreetProvider to the Dubbo
	// server. The DubboServer adapter (see server.go) depends only on this
	// function type, so the concrete service is wired here without the server
	// ever knowing about greet.GreetServiceHandler.
	gs.Provide(func() ServiceRegister {
		return func(svr *server.Server) error {
			return greet.RegisterGreetServiceHandler(svr, &GreetProvider{})
		}
	})
}

// GreetProvider implements the GreetServiceHandler interface generated from
// greet.proto.
type GreetProvider struct{}

// Greet echoes the request name back as the greeting, giving the consumer a
// deterministic value to assert on.
func (s *GreetProvider) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
	return &greet.GreetResponse{Greeting: req.Name}, nil
}
