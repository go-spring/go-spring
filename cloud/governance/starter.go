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
	"go-spring.org/cloud/governance/controlplane/admission"
	"go-spring.org/cloud/governance/controlplane/circuitbreaker"
	faultplane "go-spring.org/cloud/governance/controlplane/fault"
	loadbalanceplane "go-spring.org/cloud/governance/controlplane/loadbalance"
	"go-spring.org/cloud/governance/controlplane/ratelimit"
	"go-spring.org/cloud/governance/controlplane/timeoutretry"
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
// nothing injects it.
//
// Exported as a gs.Rooter so gs instantiates the runtime even when no client
// injects one: without a collected-type export an unreachable bean is never
// constructed, and none of the startup would run.
func init() {
	gs.Provide(&Runtime{}).
		// The control-plane roots are NOT fields (see doc.go), but they must still
		// be armed before the runtime is built, so a client that asks for an
		// executor during wiring already sees the published documents. DependsOn
		// expresses the ordering without inventing a field that would put the
		// control plane back into the runtime's object graph.
		DependsOn(
			gs.BeanIDFor[*timeoutretry.Center](),
			gs.BeanIDFor[*circuitbreaker.Center](),
			gs.BeanIDFor[*ratelimit.Center](),
			gs.BeanIDFor[*loadbalanceplane.Center](),
			gs.BeanIDFor[*admission.Center](),
			gs.BeanIDFor[*faultplane.Center](),
		).
		Export(gs.As[gs.Rooter]())
}
