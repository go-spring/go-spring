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

package fault

import (
	"go-spring.org/cloud/traffic"
)

// ServerManager is the INBOUND fault authority: it owns the inbound document,
// creates the [Server] shells server adapters fire through, and pushes a new
// kernel into them when the document changes.
//
// It is deliberately a different type from [ClientManager]. The two directions
// share the neutral kernel in core.go — rules, matching, guardrails, state
// lifecycle — but not their authority: a server starter can only reach the
// inbound object, a client starter only the outbound one. An inbound fire and an
// outbound one never see or trip each other.
//
// The call path never touches the manager: a [Server] reads its own kernel
// pointer, and the manager is reached only on creation, on Close, and on a config
// push.
type ServerManager struct {
	dir  direction
	prop traffic.Propagator // fixed at construction; shared by every shell
}

// NewServerManager returns an unarmed inbound authority: it carries no rules, so
// it injects nothing until a document is pushed.
//
// prop is the load-test convention this manager hands to its shells when a rule's
// Scope distinguishes synthetic from real traffic (required — the starter injects
// the propagator bean).
func NewServerManager(prop traffic.Propagator) *ServerManager {
	return &ServerManager{dir: newDirection(), prop: prop}
}

// Refresh adopts c as the inbound document, with the same reconciliation
// guarantees as [ClientManager.Refresh].
//
// It does not touch the outbound authority: the two sides are independent by
// design and no call ever reads both.
func (m *ServerManager) Refresh(c Config) { m.dir.refresh(c) }

// Config returns the document the inbound authority currently runs (the zero
// Config when nothing has been pushed, which is a no-op).
func (m *ServerManager) Config() Config { return m.dir.config() }

// NewServer creates an inbound shell and REGISTERS IT with m: the state
// registers itself, so the manager always perceives every state it has to push
// to. Call it once at wiring time and capture the result.
func (m *ServerManager) NewServer() *Server {
	s := &Server{dir: &m.dir, prop: m.prop}
	m.dir.register(s)
	return s
}
