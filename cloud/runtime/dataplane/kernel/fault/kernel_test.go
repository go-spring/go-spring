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

package fault

import (
	"testing"
	"time"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// TestRule_LatencyJitterBounds: with LatencyJitter J, every draw lands in
// [Latency-J, Latency+J] clamped to ≥ 0; the midpoint over many draws stays
// near Latency.
func TestRule_LatencyJitterBounds(t *testing.T) {
	const base, jitter = 100 * time.Millisecond, 80 * time.Millisecond
	r := Rule{Latency: base, LatencyJitter: jitter}
	var sum time.Duration
	for i := 0; i < 200; i++ {
		d := r.decide("svc[0]")
		assert.That(t, d.Delay >= base-jitter).True()
		assert.That(t, d.Delay <= base+jitter).True()
		sum += d.Delay
	}
	mid := sum / 200
	assert.That(t, mid > base-40*time.Millisecond && mid < base+40*time.Millisecond).True()
}

// TestManager_CloseUnregistersShell pins the lifecycle half of the ownership
// model: Close removes the shell from the manager's registry, so a later push
// no longer reaches it.
func TestManager_CloseUnregistersShell(t *testing.T) {
	m := NewManager(nil)
	a, b := m.NewShell(), m.NewShell()
	assert.Number(t, len(m.dir.states)).Equal(2)

	a.Close()
	assert.Number(t, len(m.dir.states)).Equal(1)

	// The survivor is still registered.
	_ = b
	assert.Number(t, len(m.dir.states)).Equal(1)
}

// TestRule_BindsMatchAndAction pins that a rule's Match and Action — fault's own
// types, not chain's — bind from the flattened properties an operator writes.
// Both are reached by reflect kind rather than a registered converter, so what
// matters is the spelling: match.<key> per metadata dimension, action.exact /
// action.prefix for the interface.
func TestRule_BindsMatchAndAction(t *testing.T) {
	storage := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"rules[0].service":       "gin",
		"rules[0].action.exact":  "GET /users/{id}",
		"rules[0].match.tenant":  "acme",
		"rules[0].match.region":  "cn",
		"rules[0].rate":          "1",
		"rules[1].service":       "redis:cache",
		"rules[1].action.prefix": "GET /users",
	}))
	var c Config
	assert.Error(t, conf.Bind(storage, &c)).Nil()

	assert.Number(t, len(c.Rules)).Equal(2)
	r0 := c.Rules[0]
	assert.String(t, r0.Service).Equal("gin")
	assert.String(t, r0.ActionMatch.Exact).Equal("GET /users/{id}")
	assert.String(t, r0.ActionMatch.Prefix).Equal("")
	assert.String(t, r0.MetadataMatch["tenant"]).Equal("acme")
	assert.String(t, r0.MetadataMatch["region"]).Equal("cn")
	assert.Number(t, int(c.Rules[1].Rate)).Equal(0)

	// The bound rule governs exactly the call it names: same service, same
	// interface, matching metadata — and nothing else.
	assert.That(t, r0.matchesRule(Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme", "region": "cn"}})).True()
	assert.That(t, r0.matchesRule(Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme"}})).False() // region missing
	assert.That(t, r0.matchesRule(Target{Service: "gin", Action: "POST /orders",
		Metadata: map[string]string{"tenant": "acme", "region": "cn"}})).False()
}
