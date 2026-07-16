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

// Package greet holds the WebSocket message payloads exchanged between
// provider and consumer.
//
// Unlike the sibling greet-api's types — which goctl generates from greet.api
// — go-zero's .api DSL only models request/response HTTP endpoints; WebSocket
// frames have no counterpart in it, so there is nothing to generate. These
// types are therefore hand-written, and this Go file is the "IDL", mirroring
// how the non-Triple dubbo-go examples keep their contract in proto/greet.go.
package greet

// GreetReq is a single client-to-server WebSocket message. Frames are
// JSON-encoded for symmetry with the .api HTTP endpoint of the sibling
// greet-api; nothing in go-zero forces JSON over WS — binary/text is up to
// the handler.
type GreetReq struct {
	Name string `json:"name"`
}

// GreetResp is a single server-to-client WebSocket message. Greeting echoes
// the request Name so the consumer has a deterministic value to assert on.
type GreetResp struct {
	Greeting string `json:"greeting"`
}
