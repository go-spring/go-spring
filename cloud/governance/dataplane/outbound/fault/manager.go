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
	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// Manager is the OUTBOUND fault authority: it owns the outbound document and
// publishes a new kernel when the document changes.
//
// It embeds the direction-neutral kernel manager but exposes its own type, so a
// client starter can only reach the outbound object. An outbound fire and an
// inbound one never see or trip each other.
//
// The call path never touches the manager: the executor reads the published
// kernel pointer per attempt, and the manager is reached only on creation and
// on a config push.
type Manager struct {
	*kfault.Manager
}

// NewManager returns an unarmed outbound authority: it carries no rules,
// so it injects nothing until a document is pushed.
func NewManager() *Manager {
	return &Manager{Manager: kfault.NewManager()}
}

// WrapClientExecutor returns a chain.Executor that injects faults into fn before
// delegating to next. It is the manager's object-creation half: the wrapper it
// returns lives as long as the client does, while the firing state it reads is
// the authority's current kernel, published by [Manager.Refresh].
//
// The injection happens INSIDE next's retry loop: the wrapped fn either returns
// the injected error (so the real executor retries, the breaker counts the
// failure, and observe records the outcome) or calls the real fn. This is what
// makes "setting fire" validate the resilience stack — a fault flows through
// retry/breaker/timeout/Fallback exactly as a real downstream failure would,
// rather than short-circuiting at the boundary where none of those mechanisms
// are in play.
//
// next nil makes this layer the terminal: the wrapped fn is the call itself, so
// the fire happens immediately before it (see [chain.Next]). That is how the
// composition root writes the innermost layer.
//
// The manager is the outbound authority the caller received from the container;
// it is consulted only at construction, to wire the entry point. It must be
// non-nil: the bean is always present (its constructor is REQUIRED — an absent
// bean fails startup), and with no rule armed the seam is a transparent
// pass-through anyway.
//
// The service a fault rule matches is the call's OWN: [chain.Operation.Service],
// which the client states when it builds the call (see [kfault.Target]). This
// adapter takes no label of its own — a call that names no service matches only
// the rules that name none, and quietly borrowing a label here would make a
// mislabelled call look correctly targeted. next is the executor for the same
// service, so a fault rule and a resilience rule name the same thing.
func (m *Manager) WrapClientExecutor(next chain.Executor) chain.Executor {
	return &faultExecutor{mgr: m.Manager, next: next}
}
