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

// Package logic holds the per-frame business logic for the WebSocket Greet
// endpoint. Structurally this mirrors the sibling greet-api's logic package
// — a Go-Spring bean rather than a per-request struct — so the same
// stateless GreetLogic could just as easily serve HTTP, zRPC or WS.
package logic

import (
	"context"

	"go-spring.org/spring/gs"

	"greetws/internal/types"
)

func init() {
	// Register GreetLogic as an IoC bean. ServiceContext depends on it and
	// exposes it to the WS handler, which invokes Greet once per received
	// frame.
	gs.Provide(&GreetLogic{})
}

// GreetLogic implements the Greet operation. It is stateless in this example
// but stays a struct so it can hold injected dependencies later without
// touching the handler / route wiring.
type GreetLogic struct{}

// Greet echoes the request name back as the greeting so the consumer has a
// deterministic value to assert on. Called once per WS frame; the handler
// keeps invoking it in a loop for the lifetime of the connection.
func (l *GreetLogic) Greet(ctx context.Context, req *types.GreetReq) (*types.GreetResp, error) {
	return &types.GreetResp{Greeting: req.Name}, nil
}
