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
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/gs"
)

// The center over the module authorities, plus the always-on wiring bean that
// connects gs to it (see wiring.go).
//
// The authorities themselves are NOT registered here — each is registered by the
// package that owns it (cloud/resilience, cloud/loadbalance, cloud/fault), so a
// client that injects one only has to import it. What is left for this package
// is what is specific to governance: the center, which is built over whichever
// authority beans the container holds, and the one root bean that hands it its
// Source and completes its startup.
//
// Nothing here is needed for a client to be governable-optional: a process that
// links no governance source at all still gets its authorities (from their own
// packages), only without any policy pushed into them.
func init() {
	// The center over exactly those instances: gs injects the same beans the
	// clients receive, so there is one authority per module in the process.
	gs.Provide(func(res *resilience.Manager, lb *loadbalance.Manager, inj *fault.Injector) *Center {
		return NewCenter(Config{}, res, lb, inj)
	}).Caller(1)

	// Exported as a gs.Rooter so gs collects and instantiates the wiring even
	// though no client injects it — without a collected-type export gs would
	// not instantiate an unreachable bean, so none of the startup would run.
	gs.Provide(func() *wiring { return &wiring{} }).
		Init((*wiring).Init).Destroy((*wiring).Destroy).
		Export(gs.As[gs.Rooter]()).Caller(1)
}
