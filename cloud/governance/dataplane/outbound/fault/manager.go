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
)

// Manager is the OUTBOUND fault authority: it owns the outbound document,
// exposes the [Client] entry point clients fire through, and publishes a new
// kernel when the document changes.
//
// It embeds the direction-neutral kernel manager but exposes its own type, so a
// client starter can only reach the outbound object. An outbound fire and an
// inbound one never see or trip each other.
//
// The call path never touches the manager: a [Client] reads its own kernel
// pointer, and the manager is reached only on creation and on a config push.
type Manager struct {
	*kfault.Manager
}

// NewManager returns an unarmed outbound authority: it carries no rules,
// so it injects nothing until a document is pushed.
func NewManager() *Manager {
	return &Manager{Manager: kfault.NewManager()}
}

// NewClient returns the outbound entry point an adapter fires through. It holds
// this authority and exposes only [Client.Fire], so the caller stays on the
// outbound side and cannot reach the inbound document.
func (m *Manager) NewClient() *Client {
	return &Client{mgr: m.Manager}
}
