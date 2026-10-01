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

	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// TestOpenSizesThePoolFromTheGovernanceRule proves the RESOURCE half of isolation
// reaches the pool from the rule document rather than from a per-instance key:
// the rule's max-conns wins, so the bulkhead's concurrency cap and the pool's
// connection cap are configured in one place.
//
// It is the one policy field a running executor cannot adopt — a pool is built
// once — which is why Open reads it here.
func TestOpenSizesThePoolFromTheGovernanceRule(t *testing.T) {
	mgr := resilience.NewManager()
	assert.Error(t, mgr.Apply(resilience.Settings{
		Enabled: true,
		ResolveClientPolicy: func(string) resilience.ClientPolicy {
			return resilience.ClientPolicy{MaxConns: 7}
		},
	})).Nil()

	db, err := Open(fakeDialector{}, PoolConfig{PingTimeout: time.Second}, Options{
		Engine:  "sqlite",
		Service: "gorm:sqlite:test",
		Params:  cloud.ClientParams{Resilience: mgr},
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
	db, err := Open(fakeDialector{}, PoolConfig{PingTimeout: time.Second, MaxOpenConns: 3}, Options{})
	assert.Error(t, err).Nil("open")
	defer func() { _ = db.Destroy() }()

	sqlDB, err := db.DB.DB()
	assert.Error(t, err).Nil("raw pool")
	assert.Number(t, sqlDB.Stats().MaxOpenConnections).Equal(3)
}
