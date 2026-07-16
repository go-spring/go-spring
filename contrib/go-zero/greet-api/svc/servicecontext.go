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

// Package svc holds the ServiceContext that goctl-generated route glue
// (routes.go, greethandler.go) expects, together with the GreetLogic bean it
// exposes (see logic.go). Unlike the stock go-zero scaffold — which wires
// ServiceContext up in main() and splits logic into internal/logic — here it
// is a tiny bean whose fields are injected by Go-Spring.
package svc

// ServiceContext exposes the per-endpoint logic beans to the HTTP handlers.
// It only needs to carry what the handlers use; the goctl-generated
// scaffold's Config field is intentionally omitted because Go-Spring owns
// configuration.
type ServiceContext struct {
	Logic *GreetLogic
}
