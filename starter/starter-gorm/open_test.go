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
	"testing"
	"time"

	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// testCenter returns a process governance center — an unarmed resilience manager
// (observe-only pass-through) plus an injector with no rules (transparent
// pass-through) — what a hand-built client takes now that Open receives a
// *governance.Center rather than a bundle of authorities.
func testCenter(t *testing.T) *governance.Center {
	t.Helper()
	center, err := governance.NewProcessCenter(resilience.NewManager(nil), fault.NewInjector(fault.Configs{}, nil))
	if err != nil {
		t.Fatalf("new process center: %v", err)
	}
	return center
}

// TestOpenSizesThePoolFromTheGovernanceRule proves the RESOURCE half of isolation
// reaches the pool from the rule document rather than from a per-instance key:
// the rule's max-conns wins, so the bulkhead's concurrency cap and the pool's
// connection cap are configured in one place.
//
// It is the one policy field a running executor cannot adopt — a pool is built
// once — which is why Open reads it here.
func TestOpenSizesThePoolFromTheGovernanceRule(t *testing.T) {
	center := testCenter(t)
	// Arm the rule on the manager the center exposes. It must be applied AFTER
	// the center is built: NewProcessCenter dispatches its own (empty) document on
	// construction, which would otherwise wipe a rule set on the manager first.
	assert.Error(t, center.Resilience().Apply(resilience.Settings{
		Client: resilience.ClientSettings{Default: resilience.ClientPolicy{MaxConns: 7}},
	})).Nil()

	db, err := Open(fakeDialector{}, PoolConfig{PingTimeout: time.Second}, Options{
		Engine:  "sqlite",
		Service: "gorm:sqlite:test",
		Center:  center,
	})
	assert.Error(t, err).Nil("open")
	defer func() { _ = db.Destroy() }()

	sqlDB, err := db.DB.DB()
	assert.Error(t, err).Nil("raw pool")
	assert.Number(t, sqlDB.Stats().MaxOpenConnections).Equal(7)
}

// TestOpenLeavesThePoolAloneWithoutARule proves the rule is an override and not
// a replacement: with no container, the per-instance setting still decides, so
// an app that never adopted the governance rule sees no change.
func TestOpenLeavesThePoolAloneWithoutARule(t *testing.T) {
	db, err := Open(fakeDialector{}, PoolConfig{PingTimeout: time.Second, MaxOpenConns: 3}, Options{Center: testCenter(t)})
	assert.Error(t, err).Nil("open")
	defer func() { _ = db.Destroy() }()

	sqlDB, err := db.DB.DB()
	assert.Error(t, err).Nil("raw pool")
	assert.Number(t, sqlDB.Stats().MaxOpenConnections).Equal(3)
}
