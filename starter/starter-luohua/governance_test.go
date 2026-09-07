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

package luohua

import (
	"context"
	"testing"

	"go-spring.org/cloud/governance/resilience"
)

// TestGovernanceDriverRegisteredAndFlavored locks the luohua governance
// capability: importing luohua registers a "luohua" resilience backend (a real,
// verifiable extension — a distinct luohua executor over the bundled engine),
// selectable by govern.driver=luohua. This is the observable-flavor rule: an
// extension with no verifiable behavior would silently pass through and a
// mis-wire would go unnoticed.
func TestGovernanceDriverRegisteredAndFlavored(t *testing.T) {
	d, err := resilience.GetDriver("luohua")
	if err != nil {
		t.Fatalf("luohua resilience driver not registered: %v", err)
	}

	ex, err := d.NewExecutor(resilience.Policy{})
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	lh, ok := ex.(*luohuaExecutor)
	if !ok {
		t.Fatalf("executor is %T, want *luohuaExecutor (the luohua flavor over the bundled engine)", ex)
	}
	if lh.inner == nil {
		t.Fatal("luohua executor should wrap the bundled default engine, not invent a new one")
	}

	ran := false
	if err := ex.Execute(context.Background(), "demo:resource", func(context.Context) error {
		ran = true
		return nil
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !ran {
		t.Fatal("Execute did not delegate to the inner engine")
	}
}
