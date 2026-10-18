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
	kfault "go-spring.org/cloud/runtime/dataplane/kernel/fault"
	"go-spring.org/cloud/runtime/dataplane/kernel/traffic"
)

// ServerManager is the INBOUND fault authority: it owns the inbound document,
// creates the [Server] shells server adapters fire through, and pushes a new
// kernel into them when the document changes.
//
// It embeds the direction-neutral kernel manager but exposes its own type, so a
// server starter can only reach the inbound object. An inbound fire and an
// outbound one never see or trip each other.
//
// The call path never touches the manager: a [Server] reads its own kernel
// pointer, and the manager is reached only on creation, on Close, and on a
// config push.
type ServerManager struct {
	*kfault.Manager
}

// NewServerManager returns an unarmed inbound authority: it carries no rules, so
// it injects nothing until a document is pushed.
//
// prop is the load-test convention this manager hands to its shells when a rule's
// Scope distinguishes synthetic from real traffic.
func NewServerManager(prop traffic.Propagator) *ServerManager {
	return &ServerManager{Manager: kfault.NewManager(prop)}
}

// NewServer creates an inbound shell and REGISTERS IT with the authority: the
// state registers itself, so the manager always perceives every state it has to
// push to. Call it once at wiring time and capture the result.
func (m *ServerManager) NewServer() *Server {
	return &Server{Shell: m.Manager.NewShell()}
}
