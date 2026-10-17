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
	center, err := governance.NewProcessCenter(resilience.NewClientManager(resilience.NewDefaultDriver(nil)), fault.NewClientManager(nil), fault.NewServerManager(nil))
	if err != nil {
		t.Fatalf("new process center: %v", err)
	}
	return center
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
