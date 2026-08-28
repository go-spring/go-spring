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

package StarterMQTT

import (
	"testing"

	observe "go-spring.org/cloud/observe"
)

// resetObsCfgForTest restores the seed state so the seeding tests are
// order-independent and leave the package defaults in place for other tests.
func resetObsCfgForTest(t *testing.T) {
	t.Helper()
	obsCfgMu.Lock()
	obsCfg = observe.ObserveConfig{Level: observe.DefaultBrief}
	obsCfgSeeded = false
	obsCfgMu.Unlock()
}

// TestSeedObserveConfigFirstWins verifies the span-helper observer seeding
// rule: the first configured client's observability config wins, later clients
// (which may carry a different level) do not overwrite it, and a level set
// before any seeding reaches the observer construction.
func TestSeedObserveConfigFirstWins(t *testing.T) {
	defer resetObsCfgForTest(t)
	seedObserveConfig(observe.ObserveConfig{Level: "off"})
	seedObserveConfig(observe.ObserveConfig{Level: "detailed"})

	obsCfgMu.Lock()
	defer obsCfgMu.Unlock()
	if obsCfg.Level != "off" {
		t.Fatalf("first-seeded level should win: want off, got %q", obsCfg.Level)
	}
}

// TestSeedObserveConfigEmptyKeepsDefault verifies an unconfigured client
// (empty level) does not clobber the kit default the helpers fall back to.
func TestSeedObserveConfigEmptyKeepsDefault(t *testing.T) {
	defer resetObsCfgForTest(t)
	seedObserveConfig(observe.ObserveConfig{})
	obsCfgMu.Lock()
	defer obsCfgMu.Unlock()
	if obsCfg.Level != observe.DefaultBrief {
		t.Fatalf("empty seed should keep the default: want %q, got %q", observe.DefaultBrief, obsCfg.Level)
	}
}
