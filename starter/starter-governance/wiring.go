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
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// This file is the WIRING between gs and the container-free governance core
// (cloud/governance). Blank-importing starter-governance registers the module
// authorities and the center over them as beans, then one root bean that hands
// the center its governance.Source and completes its startup.
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

func init() {
	// The three module authorities, one bean each.
	gs.Provide(func() *resilience.Manager { return resilience.NewManager() }).Caller(1)
	gs.Provide(func() *loadbalance.Manager { return loadbalance.NewManager() }).Caller(1)
	gs.Provide(func() *fault.Injector { return fault.NewInjector(fault.Configs{Client: fault.Config{}}, nil) }).Caller(1)

	// The bundled engine, contributed as a NAMED driver bean. It takes the
	// process's counter store IF a backend starter contributed one — a Redis
	// store, say, which is what puts a rate limit beyond the process and onto
	// every replica. With no such bean the injection is nil and each executor
	// counts in a budget of its own, which still means one budget per
	// service: the manager builds one executor per label, shared by every caller
	// of it. A process that configures driver=sentinel still gets this bean; it
	// simply is not the one selected.
	gs.Provide(func(c resilience.Counters) resilience.Driver { return resilience.NewDefaultDriver(c) }, gs.TagArg("?")).
		Name(resilience.DefaultDriverName).
		Export(gs.As[resilience.Driver]()).Caller(1)

	// The center over exactly those instances: gs injects the same beans the
	// clients receive, so there is one authority per module in the process.
	gs.Provide(func(res *resilience.Manager, lb *loadbalance.Manager, inj *fault.Injector) *governance.Center {
		return governance.NewCenter(governance.Config{}, res, lb, inj)
	}).Caller(1)

	// Exported as a gs.Rooter so gs collects and instantiates the wiring even
	// though no client injects it — without a collected-type export gs would
	// not instantiate an unreachable bean, so none of the startup would run.
	gs.Provide(func() *wiring { return &wiring{} }).
		Init((*wiring).Init).Destroy((*wiring).Destroy).
		Export(gs.As[gs.Rooter]()).Caller(1)
}

// wiring is the always-registered bean that connects gs to the governance
// center. It holds the center, the optional bean-injected source, and the
// driver backends the container contributed.
type wiring struct {
	// Ctr is the governance center bean this package registers above.
	Ctr *governance.Center `autowire:"?"`

	// Src is the source gs field-injects here: any bean exported as a
	// governance.Source (autowire:"?" is nullable — no such bean leaves it nil
	// and governance stays disabled). Source priority is an explicit
	// Center.SetSource (any time) > Src.
	Src governance.Source `autowire:"?"`

	// Drivers is every resilience backend bean in the container, keyed by bean
	// name — including this package's "default" (see init). A backend
	// contributes one with
	// gs.Provide(...).Name("<backend>").Export(gs.As[resilience.Driver]()); the
	// Export is required, since gs indexes beans by their exact type and an
	// unexported concrete driver would be invisible here. Naming a driver that
	// is not in this map fails startup (Center.SetDrivers → GoLive).
	Drivers map[string]resilience.Driver `autowire:"?"`
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
		return errutil.Explain(nil, "starter-governance: governance center bean is missing")
	}
	// Drivers first: GoLive validates the configured driver name against the
	// directory, so the directory must be in place before the center goes live.
	// A nil map is a no-op (the manager keeps its bundled driver).
	w.Ctr.SetDrivers(w.Drivers)
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
