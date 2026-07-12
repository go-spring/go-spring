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

// Package svc holds the ServiceContext that the WS handler expects.
// Structurally mirrors the sibling greet-api's svc package: a tiny bean whose
// fields are injected by Go-Spring, holding whatever per-endpoint logic the
// handler needs.
package svc

import "greetws/internal/logic"

// ServiceContext exposes the per-endpoint logic beans to the WS handler. It
// only needs to carry what the handler uses; there is no Config field because
// Go-Spring owns configuration.
type ServiceContext struct {
	Logic *logic.GreetLogic
}
