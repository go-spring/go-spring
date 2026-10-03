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
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

// The center over the module authorities — the one bean this package registers,
// and with it the whole of the gs wiring between this package and its own
// service-governance core.
//
// The authorities themselves are NOT registered here — each is registered by the
// package that owns it (cloud/resilience, cloud/loadbalance, cloud/fault), so a
// client that injects one only has to import it. What is left for this package
// is what is specific to governance: the center, built over whichever authority
// beans the container holds, plus the two lifecycle hooks that put it to work.
//
// The authorities are beans for two reasons at once: the center drives them, and
// every client injects them by concrete type. That is what replaced the neutral
// process-wide seams the core used to publish — same inversion (the module owns
// the contract, the center is the implementation), but reached through the
// container instead of a package-level pointer, so there is one mechanism rather
// than two.
//
// Governance's configuration is its OWN system — a rules file watched by the
// file source, a remote console via the http source, or any other Source — and
// deliberately NOT bound through gs properties: a governance rule change
// refreshes governance only, never re-binds the whole app.
//
// Nothing here is needed for a client to be governable-optional: a process that
// links no governance source at all still gets its authorities (from their own
// packages), only without any policy pushed into them.
func init() {
	// The center over exactly those instances: gs injects the same beans the
	// clients receive, so there is one authority per module in the process. The
	// discovery manager joins them so a client reaches discovery through the
	// center like every other authority — it is not a policy authority (the
	// center does not apply rules to it), only held and handed out.
	//
	// The source arrives here too, and it is REQUIRED: a container that has this
	// package (any client or server starter links it) must also hold a Source
	// bean — file/http/nacos/…, whatever the deployment's rules come from. A
	// process that contributes none fails startup on this very parameter rather
	// than running with governance silently off. It is a constructor parameter
	// rather than a field installed later because the center must be born with
	// it: nothing then has to arbitrate between "the bean's source" and "a
	// caller's SetSource", so the rule reduces to the plain last-write-wins of
	// [Center.SetSource].
	//
	// The bean IS the startup: [Center.GoLive] is its Init hook and
	// [Center.Close] its Destroy hook, so there is no separate wiring bean whose
	// only job would be to call them. That also settles the ordering for anything
	// that depends on the center — gs wires it before its dependents, so a Rooter
	// that injects it never observes a center that has not gone live.
	//
	// Exported as a gs.Rooter so gs instantiates the center even when no client
	// injects one: without a collected-type export an unreachable bean is never
	// constructed, and none of the startup would run.
	gs.Provide(func(res *resilience.Manager, lb *loadbalance.Manager, inj *fault.Injector,
		disc *discovery.Manager, src Source) *Center {
		return NewCenter(Config{}, res, lb, inj, disc, src)
	}).
		Init((*Center).GoLive).Destroy((*Center).Close).
		Export(gs.As[gs.Rooter]()).Caller(1)

	// The two built-in source adapters, registered by the package that owns
	// them: linking cloud/governance is enough to get the file and http sources,
	// with no separate starter to import. One source per process (the center
	// holds exactly one active source), so each is a plain conditional bean, not
	// a Group; OnProperty is a prefix check, so any
	// spring.governance.source.<kind>.* key arms its module.
	//
	// Both are exported as [Source], which is what the center's constructor
	// collects. Without the Export the bean is invisible to interface injection
	// and startup fails on that required parameter — the most likely
	// misconfiguration, called out in the README.
	gs.Module(gs.OnProperty("spring.governance.source.file"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c fileSourceConfig
		if err := conf.Bind(p, &c, "${spring.governance.source.file:=}"); err != nil {
			return err
		}
		r.Provide(func() (*FileSource, error) { return NewFileSource(c.Path) }).
			Init((*FileSource).Init).Destroy((*FileSource).Close).
			Export(gs.As[Source]()).Caller(1)
		return nil
	})

	// "http": poll a governance console. Same registration shape as "file".
	gs.Module(gs.OnProperty("spring.governance.source.http"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c httpSourceConfig
		if err := conf.Bind(p, &c, "${spring.governance.source.http:=}"); err != nil {
			return err
		}
		r.Provide(func() (*HTTPSource, error) { return NewHTTPSource(c.URL, c.Interval, c.Format, c.Headers) }).
			Init((*HTTPSource).Init).Destroy((*HTTPSource).Close).
			Export(gs.As[Source]()).Caller(1)
		return nil
	})
}

var starterTag = log.RegisterAppTag("governance", "")
