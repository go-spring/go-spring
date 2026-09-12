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

package gormcore

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
)

// fakeSelectionProvider stands in for the governance-backed provider, recording
// the label the starter bound and letting a test push a policy. It is the
// documented way to drive the seam from a test: RegisterSelectionProvider(nil)
// disarms it again, so no global leaks into another test.
type fakeSelectionProvider struct {
	labels []string
	apply  func(loadbalance.Selection)
	detach int
}

func (f *fakeSelectionProvider) install(t *testing.T) {
	t.Helper()
	loadbalance.RegisterSelectionProvider(func(label string, apply func(loadbalance.Selection)) func() {
		f.labels = append(f.labels, label)
		f.apply = apply
		return func() { f.detach++ }
	})
	t.Cleanup(func() { loadbalance.RegisterSelectionProvider(nil) })
}

// TestNewPickPoolBindsSelection covers the gorm half of endpoint selection: the
// shared pool builder attaches a suspension tracker (without one the thresholds
// land on nothing) and binds the pool to the entry's resource label, so one
// govern.rules[N] rule — the same label that drives the entry's protection
// executor — also governs its balancing strategy. The returned stop detaches.
func TestNewPickPoolBindsSelection(t *testing.T) {
	f := &fakeSelectionProvider{}
	f.install(t)

	c := Common{ServiceName: "user-db", Scheme: "tcp"}
	backend := discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:3306", Healthy: true, Weight: 1})

	const label = "gorm:mysql:user-db"
	pool, resolver, stop, err := c.NewPickPool(context.Background(), backend, label)
	assert.Error(t, err).Nil()
	if resolver == nil || pool == nil {
		t.Fatal("service-name set with a backend bean should build a discovery pool")
	}

	assert.That(t, f.labels).Equal([]string{label})
	assert.That(t, pool.Tracker()).NotNil()

	// The policy the provider pushes reaches the pool's selection.
	f.apply(loadbalance.Selection{Balancer: loadbalance.LeastConn, OutlierThreshold: 3, OutlierSuspendFor: time.Minute})
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(3)

	stop()
	assert.Number(t, f.detach).Equal(1)
}

// TestNewPickPoolDirectAddrBindsNothing pins that a client dialing a fixed
// address has no candidate set to choose from: no pool, no subscription, and a
// stop that is safe to call anyway.
func TestNewPickPoolDirectAddrBindsNothing(t *testing.T) {
	f := &fakeSelectionProvider{}
	f.install(t)

	c := Common{}
	pool, resolver, stop, err := c.NewPickPool(context.Background(), nil, "gorm:mysql:127.0.0.1:3306")
	assert.Error(t, err).Nil()
	assert.That(t, pool == nil).True()
	assert.That(t, resolver == nil).True()
	assert.Number(t, len(f.labels)).Equal(0)

	stop()
}
