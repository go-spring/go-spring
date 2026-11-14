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
	"context"

	kfault "go-spring.org/cloud/governance/capability/fault"
)

// Manager is the INBOUND fault authority: it owns the inbound document, its
// [ServerRule] list and the injection kernel, exposes the [Server] entry point
// server adapters fire through, and publishes a new kernel when the document
// changes.
//
// It is a SEPARATE object and a SEPARATE configuration type from the outbound
// authority: a server starter can only reach this one, an inbound fire and an
// outbound one never see or trip each other, and the two directions cannot
// express each other's effects.
//
// The call path never touches the manager: a [Server] reads its own kernel
// pointer, and the manager is reached only on creation and on a config push.
type Manager struct {
	kernel *kfault.Kernel[ServerConfig]
}

// NewManager returns an unarmed inbound authority: it carries no rules, so it
// injects nothing until a document is pushed.
func NewManager() *Manager {
	return &Manager{kernel: kfault.NewKernel[ServerConfig]()}
}

// Refresh adopts c as this direction's document, then publishes the new kernel.
// Callers must serialize pushes (the center's source loop already does); a
// request reads the kernel when it starts, so a toggle takes effect on the next
// request.
//
// c is ALREADY validated: every expr tag runs where the document enters —
// conf.Bind calls it for every bound field, and a source rejects a document that
// fails it before pushing. Callers that construct a ServerConfig in code must
// validate it themselves before committing.
func (m *Manager) Refresh(c ServerConfig) {
	m.kernel.Refresh(c, c.RunID, toRules(c.Rules))
}

// Config returns the document this authority currently runs — the cfg inside the
// published kernel, so there is exactly one copy of the document (the zero
// ServerConfig when nothing has been pushed, which is a no-op).
func (m *Manager) Config() ServerConfig {
	return m.kernel.Config()
}

// Fire evaluates one inbound request against the current kernel. See
// [kfault.Kernel.Fire] for the shared sequence; the inbound rule supplies the
// failure the adapter renders.
func (m *Manager) Fire(ctx context.Context, t kfault.Target) (kfault.Result, error) {
	return m.kernel.Fire(ctx, t)
}

// NewServer returns the inbound entry point an adapter fires through. It holds
// this authority and exposes only [Server.Fire], so the caller stays on the
// inbound side and cannot reach the outbound document. Call it once at wiring
// time and capture the result.
func (m *Manager) NewServer() *Server {
	return &Server{mgr: m}
}
