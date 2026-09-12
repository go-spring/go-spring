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

package StarterGovernance

import (
	"go-spring.org/cloud/governance"
	"go-spring.org/spring/gs"
)

// This file is the WIRING between gs and the container-free governance core
// (cloud/governance). Blank-importing starter-governance registers one root
// bean that hands the governance center its governance.Source and completes the
// authority's startup. Governance's configuration is its OWN system — a rules
// file watched by this module's file source, a remote console via the http
// source, or any other Source — and deliberately NOT bound through gs
// properties: a governance rule change refreshes governance only, never
// re-binds the whole app.

// wiring is the always-registered bean that connects gs to the governance
// singleton. It holds the optional bean-injected source.
type wiring struct {
	// Src is the source gs field-injects here: any bean exported as a
	// governance.Source (autowire:"?" is nullable — no such bean leaves it nil
	// and governance stays disabled unless something calls SetSource). Source
	// priority is an explicit governance.SetSource (any time) > Src.
	Src governance.Source `autowire:"?"`
}

func init() {
	// Exported as a gs.Rooter so gs collects and instantiates the wiring even
	// though no client injects it — without a collected-type export gs would
	// not instantiate an unreachable bean, so none of the registrations fire.
	gs.Provide(newWiring).
		Init((*wiring).Init).Destroy((*wiring).Destroy).
		Export(gs.As[gs.Rooter]()).Caller(1)
}

func newWiring() *wiring { return &wiring{} }

// Init binds the bean-injected source and completes the authority's startup. An
// explicit governance.SetSource called before wiring wins — BindDefault is a
// no-op when a source is already bound, and a no-op on a nil source, so a
// process with no source beans simply leaves the center disabled (every client
// resolves a transparent pass-through).
//
// GoLive runs unconditionally: it registers the executor/fault seams and fires
// OnReady whether or not a source was bound.
func (w *wiring) Init() error {
	governance.BindDefault(w.Src)
	governance.GoLive()
	return nil
}

// Destroy closes the active source when it happens to be closeable (the
// Source contract keeps Close optional), via the governance facade.
func (w *wiring) Destroy() error { return governance.CloseActiveSource() }
