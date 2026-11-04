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
	"go-spring.org/spring/gs"
)

// The runtime bean: the process's single data-plane injection point.
//
// Every authority it holds is registered by the package that owns it — the
// outbound resilience manager by cloud/governance/dataplane/outbound/resilience,
// the inbound admission manager by cloud/governance/dataplane/inbound/admission,
// endpoint selection by cloud/governance/capability/loadbalance, fault
// injection by the two fault packages, traffic and discovery by their kernel
// packages. This package only declares the runtime over them, so adding an
// authority never touches this file.
//
// The control plane is NOT among them: limits, breaker, rate limit, selection,
// admission and fault are peer roots (see doc.go), each registered by its own
// package and exported as a gs.Rooter so gs constructs and arms it even though
// nothing injects it. Their EXISTENCE is the deployment's declaration, not this
// package's: gs has no classpath scan — Go's import graph is the component
// scan — so a root is linked in by whoever feeds it (the governance source
// starters, or the app itself), and this package deliberately aggregates none.
//
// Exported as a plain bean, not a gs.Rooter: the runtime's construction has no
// side effect (the authorities register and arm themselves in their own
// packages), so a deployment where nothing injects a runtime loses nothing by
// not building one.
func init() {
	// No DependsOn on the control-plane roots: construction order against
	// this runtime is deliberately unconstrained, because every authority the
	// runtime holds tolerates either order — an executor reads live snapshots
	// per call, and a center's Apply pushes to whatever is registered by the
	// time it runs. Ordering here would guard a constraint the data plane does
	// not have.
	gs.Provide(&Runtime{})
}
