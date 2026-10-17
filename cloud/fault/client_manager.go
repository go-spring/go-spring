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

// ClientManager is the OUTBOUND fault authority: it owns the outbound document,
// creates the [Client] shells clients fire through, and pushes a new kernel into
// them when the document changes.
//
// It is deliberately a different type from [ServerManager]. The two directions
// share the neutral kernel in core.go — rules, matching, guardrails, state
// lifecycle — but not their authority: a client starter can only reach the
// outbound object, a server starter only the inbound one. An outbound fire and an
// inbound one never see or trip each other.
//
// The call path never touches the manager: a [Client] reads its own kernel
// pointer, and the manager is reached only on creation, on Close, and on a config
// push.
type ClientManager struct {
	dir  direction
	prop traffic.Propagator // fixed at construction; shared by every shell
}

// NewClientManager returns an unarmed outbound authority: it carries no rules,
// so it injects nothing until a document is pushed.
//
// prop is the load-test convention this manager hands to its shells when a rule's
// Scope distinguishes synthetic from real traffic (required — the starter injects
// the propagator bean).
func NewClientManager(prop traffic.Propagator) *ClientManager {
	return &ClientManager{dir: newDirection(), prop: prop}
}

// Refresh adopts c as the outbound document: it rebuilds the kernel, reusing the
// guardrail counters of rules that are still the same experiment and dropping the
// state of rules that are gone (see [buildCore]). It is safe to call concurrently
// with a gated call; each call observes the kernel current at its own load, so a
// toggle takes effect on the next operation.
//
// It does not touch the inbound authority: the two sides are independent by
// design and no call ever reads both.
func (m *ClientManager) Refresh(c Config) { m.dir.refresh(c) }

// Config returns the document the outbound authority currently runs (the zero
// Config when nothing has been pushed, which is a no-op).
func (m *ClientManager) Config() Config { return m.dir.config() }

// NewClient creates an outbound shell and REGISTERS IT with m: the state
// registers itself, so the manager always perceives every state it has to push
// to. Create one per consumer and close it when the consumer goes away.
func (m *ClientManager) NewClient() *Client {
	c := &Client{dir: &m.dir, prop: m.prop}
	m.dir.register(c)
	return c
}
