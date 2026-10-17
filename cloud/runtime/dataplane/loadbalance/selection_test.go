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

package loadbalance

import (
	"context"
	"sync"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// fakeSource stands in for the governance center: it holds the selection the
// current document carries as its Default and lets a test push a later change
// through [Manager.Apply], which is the only entry point the center uses.
type fakeSource struct {
	mu  sync.Mutex
	cur Selection
}

// settings renders the source's current document as pushed settings.
func (f *fakeSource) settings() Settings {
	f.mu.Lock()
	defer f.mu.Unlock()
	return Settings{Default: f.cur}
}

// manager returns a Manager armed with this source's current selection.
func (f *fakeSource) manager(t *testing.T) *Manager {
	m := newManager(t)
	m.Apply(context.Background(), f.settings())
	return m
}

// push adopts s as the source's current selection and re-applies, delivering the
// change to every registered chain.
func (f *fakeSource) push(m *Manager, s Selection) {
	f.mu.Lock()
	f.cur = s
	f.mu.Unlock()
	m.Apply(context.Background(), f.settings())
}

// selectionFor reads what label resolves to, under the manager lock. The Manager
// deliberately exposes no read-back for this — production only ever pushes into
// sinks (see [Manager.Chain]) — so the matcher is exercised
// here directly, in-package.
func selectionFor(m *Manager, label string) Selection {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.selectionForLocked(label)
}

// TestUnarmedResolvesZero pins the matcher's read path on an unarmed manager.
func TestUnarmedResolvesZero(t *testing.T) {
	m := newManager(t)
	assert.That(t, selectionFor(m, "demo:service")).Equal(Selection{})
}

// TestSettingsMatch pins the matching this package now owns (it used to live in
// the center's document walk): an exact label match wins, an unmatched label
// falls to Default, a rule with an empty Service never captures a real label (it
// is not a catch-all — Default is how a document says "everything else"), and
// the first matching Rule wins.
func TestSettingsMatch(t *testing.T) {
	rule := Selection{Balancer: LeastConn, OutlierThreshold: 2}
	def := Selection{Balancer: P2C}
	shadowed := Selection{Balancer: Weighted}

	m := newManager(t)
	m.Apply(context.Background(), Settings{
		Default: def,
		Rules: []Rule{
			{Service: "", Selection: shadowed}, // matches no real label
			{Service: "demo:service", Selection: rule},
			{Service: "demo:service", Selection: shadowed}, // shadowed: first match wins
		},
	})

	assert.That(t, selectionFor(m, "demo:service")).Equal(rule)
	assert.That(t, selectionFor(m, "other:service")).Equal(def)
}
