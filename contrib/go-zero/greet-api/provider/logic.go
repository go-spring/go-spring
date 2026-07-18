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

// GreetLogic (below) holds the business logic for each Greet endpoint. In a
// stock go-zero project it is scaffolded by `goctl api go` under
// internal/logic and edited in place; here we replace the scaffold with a
// Go-Spring bean, colocated with the ServiceContext that exposes it so the
// logic participates in DI just like every other Go-Spring component.
package main

import (
	"context"

	"go-spring.org/spring/gs"

	"greetapi/idl"
)

func init() {
	// Register GreetLogic as an IoC bean. ServiceContext depends on it and
	// exposes it to the generated handler.
	gs.Provide(&GreetLogic{})
}

// GreetLogic implements the Greet endpoint. It is stateless in this example
// but stays a struct so it can hold injected dependencies later without
// touching the handler / route wiring.
type GreetLogic struct{}

// Greet echoes the request name back as the greeting so the consumer has a
// deterministic value to assert on.
func (l *GreetLogic) Greet(ctx context.Context, req *idl.GreetReq) (*idl.GreetResp, error) {
	return &idl.GreetResp{Greeting: req.Name}, nil
}
