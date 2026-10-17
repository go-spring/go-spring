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

package runtime

import (
	"go-spring.org/cloud/runtime/controlplane/governance"
	"go-spring.org/cloud/runtime/dataplane/discovery"
	"go-spring.org/cloud/runtime/dataplane/fault"
	"go-spring.org/cloud/runtime/dataplane/loadbalance"
	"go-spring.org/cloud/runtime/dataplane/resilience"
	"go-spring.org/spring/gs"
)

// The runtime over the module authorities — the one bean this package registers,
// and with it the whole of the gs wiring between the control plane, the data
// plane and the process.
//
// The authorities themselves are NOT registered here — each is registered by
// the package that owns it (cloud/runtime/dataplane/resilience,
// cloud/runtime/dataplane/loadbalance, cloud/runtime/dataplane/fault,
// cloud/runtime/dataplane/discovery), so a client that injects one only has to
// import it. What is left for this package is the composition: build one
// [Runtime] over whichever authority beans the container holds, arm the
// authorities with the initial document, and expose the runtime as the single
// injection point.
func init() {
	// The source arrives here too, and it is REQUIRED: a container that has this
	// package (any client or server starter links it) must also hold a
	// [governance.Source] bean — file/http/nacos/…, whatever the deployment's
	// rules come from. A process that contributes none fails startup on this very
	// parameter rather than running with no governance document at all. It is a
	// constructor parameter rather than a field installed later because the
	// runtime must be born with it.
	//
	// The nil fallback below is for the container's NULLABLE wiring mode
	// (spring.force-autowire-is-nullable, which gs.RunTest forces on): a missing
	// bean resolves to nil there instead of failing, so the bean normalizes it to
	// an empty source. That keeps [New]'s contract strict — src is never nil —
	// while the fail-loud behavior of a real deployment is untouched: with
	// nullable off, gs rejects the missing bean before this constructor runs.
	//
	// The bean's construction IS the startup: [New] arms the authorities with the
	// initial document and fails on a document that cannot serve, and
	// [Runtime.Close] is the Destroy hook — so there is no separate wiring bean
	// and no Init hook whose only job would be to call them. That also settles
	// the ordering for anything that depends on the runtime — gs wires it before
	// its dependents, so a dependent never observes an unarmed runtime.
	//
	// Exported as a gs.Rooter so gs instantiates the runtime even when no client
	// injects one: without a collected-type export an unreachable bean is never
	// constructed, and none of the startup would run.
	gs.Provide(func(resClient *resilience.ClientManager,
		lb *loadbalance.Manager, injClient *fault.ClientManager, injServer *fault.ServerManager,
		disc *discovery.Manager, limiter resilience.ClusterLimiter, src governance.Source) (*Runtime, error) {
		if src == nil { // nullable wiring only; see above
			src = governance.NewPushSource(governance.Config{})
		}
		var opts []Option
		if limiter != nil {
			opts = append(opts, WithClusterLimiter(limiter))
		}
		return New(resClient, lb, injClient, injServer, disc, src, opts...)
	}, gs.IndexArg(5, gs.TagArg("?"))).
		Destroy((*Runtime).Close).
		Export(gs.As[gs.Rooter]()).Caller(1)
}
