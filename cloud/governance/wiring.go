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

package governance

import (
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/errutil"
)

// This file is the wiring between gs and this package's own service-governance
// core. Importing cloud/governance runs the registrations in starter.go — the module
// authorities and the center over them, plus one root bean that hands the center
// its governance.Source and completes its startup — so every container that
// links this package has the authorities, without a separate starter to import.
//
// The authorities are beans for two reasons at once: the wiring drives them, and
// every client injects them by concrete type. That is what replaced the neutral
// process-wide seams the core used to publish — same inversion (the module owns
// the contract, the center is the implementation), but reached through the
// container instead of a package-level pointer, so there is one mechanism rather
// than two.
//
// Governance's configuration is its OWN system — a rules file watched by this
// module's file source, a remote console via the http source, or any other
// Source — and deliberately NOT bound through gs properties: a governance rule
// change refreshes governance only, never re-binds the whole app.

// wiring is the always-registered bean that connects gs to the center. It
// holds the center, the optional bean-injected source, and the driver backends
// the container contributed.
type wiring struct {
	// Ctr is the center bean starter.go registers.
	Ctr *Center `autowire:"?"`

	// Src is the source gs field-injects here: any bean exported as a
	// Source (autowire:"?" is nullable — no such bean leaves it nil
	// and governance stays disabled). Source priority is an explicit
	// Center.SetSource (any time) > Src.
	Src Source `autowire:"?"`

	// Drivers is every resilience backend bean in the container, keyed by bean
	// name — including this package's "default" (see init). A backend
	// contributes one with
	// gs.Provide(...).Name("<backend>").Export(gs.As[resilience.Driver]()); the
	// Export is required, since gs indexes beans by their exact type and an
	// unexported concrete driver would be invisible here. Naming a driver that
	// is not in this map fails startup (Center.SetDrivers → GoLive).
	Drivers map[string]resilience.Driver `autowire:"?"`

	// Factories is every load-balancing strategy bean in the container, keyed by
	// bean name — the directory a rule's `balancer` name is resolved against,
	// beside the built-in strategies. A deployment contributes one with
	// gs.Provide(...).Name("<strategy>").Export(gs.As[loadbalance.Factory]());
	// the Export is required for the same reason as Drivers. A name that
	// shadows a built-in strategy fails startup (Center.SetBalancerFactories).
	Factories map[string]loadbalance.Factory `autowire:"?"`
}

// Init binds the bean-injected source and completes the center's startup. An
// explicit Center.SetSource called before wiring wins — BindDefault is a
// no-op when a source is already bound, and a no-op on a nil source, so a
// process with no source beans simply leaves the center disabled (every client
// resolves a transparent pass-through).
//
// GoLive runs unconditionally: it distributes the snapshot to the module
// authorities and fires OnReady whether or not a source was bound.
func (w *wiring) Init() error {
	if w.Ctr == nil {
		return errutil.Explain(nil, "governance: center bean is missing")
	}
	// Directories first: GoLive validates the configured driver name against the
	// resilience one, so both must be in place before the center goes live. A nil
	// map is a no-op in each case — the manager keeps its bundled driver, the
	// pool keeps the built-in strategies.
	w.Ctr.SetDrivers(w.Drivers)
	if err := w.Ctr.SetBalancerFactories(w.Factories); err != nil {
		return err
	}
	w.Ctr.BindDefault(w.Src)
	return w.Ctr.GoLive()
}

// Destroy closes the active source when it happens to be closeable (the
// Source contract keeps Close optional).
func (w *wiring) Destroy() error {
	if w.Ctr == nil {
		return nil
	}
	return w.Ctr.Close()
}
