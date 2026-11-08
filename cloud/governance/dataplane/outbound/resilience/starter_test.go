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

package resilience

import (
	"context"
	"testing"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/spring/gs"
)

// managerProbe exists only to make gs instantiate the manager bean and hand it
// to the test.
type managerProbe struct{}

// TestStarter_ManagerBeanIsInjected pins the starter registration: a client
// starter that injects *resilience.Manager gets the bean, and an unarmed one is
// a transparent pass-through.
func TestStarter_ManagerBeanIsInjected(t *testing.T) {
	var m *Manager

	gs.Web(false).Configure(func(app gs.App) {
		// A stand-in for a client starter: it injects the manager bean so gs
		// instantiates it, and captures it for the test. Exported as a Rooter
		// because nothing autowires it.
		app.Provide(func(mgr *Manager) *managerProbe {
			m = mgr
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if m == nil {
			t.Fatal("the manager bean was not injected")
		}
		e := m.Wrap("res:1", nil)
		if err := e.Execute(context.Background(), chain.Operation{}, new(chain.Recorder), func(context.Context) error { return nil }); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
}
