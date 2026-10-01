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

package StarterBigCache

import (
	"testing"

	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/gs"
)

// governanceProbe takes the four governance beans as OPTIONAL injections. Under
// gs.RunTest every injection is forced nullable, so a nil field here means the
// bean is genuinely absent from the container — which is exactly what this test
// needs to distinguish "registered" from "degraded to nil".
type governanceProbe struct {
	Mgr  *resilience.Manager  `autowire:"?"`
	Lb   *loadbalance.Manager `autowire:"?"`
	Inj  *fault.Injector      `autowire:"?"`
	Seen bool
}

// TestImportsProvideGovernanceBeans pins the contract this starter's imports
// exist for: importing a client starter ALONE puts the governance beans in the
// container, so a deployment gets a governable client without knowing anything
// about governance. Each bean is registered by the package that owns it
// (cloud/resilience, cloud/loadbalance, cloud/fault), and every client starter
// imports those packages for the injected types — so there is no separate
// governance import to forget. Turning governance off is
// spring.governance.enabled=false (or binding no rule source), not the absence
// of a bean.
//
// The injected parameters stay nullable precisely so this is a one-way door: if
// a client starter ever stops importing one of those packages, the field goes
// nil and this test fails, instead of the whole family silently degrading to
// pass-through.
func TestImportsProvideGovernanceBeans(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		// Nothing is registered here on purpose: whatever the container holds it
		// holds because this package's imports put it there.
	}).RunTest(t, func(p *governanceProbe) {
		if p.Mgr == nil {
			t.Fatal("importing starter-bigcache must provide the *resilience.Manager bean")
		}
		if p.Lb == nil {
			t.Fatal("importing starter-bigcache must provide the *loadbalance.Manager bean")
		}
		if p.Inj == nil {
			t.Fatal("importing starter-bigcache must provide the *fault.Injector bean")
		}
	})
}
