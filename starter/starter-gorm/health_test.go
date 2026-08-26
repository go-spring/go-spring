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

	"go-spring.org/stdlib/testing/assert"
	"time"

	"go-spring.org/cloud/actuator/health"
)

// TestGormHealth pins the indicator contract: the name joins prefix+instance,
// CheckHealth delegates to the pool ping, and a broken backend surfaces as an
// error (DOWN) rather than a panic.
func TestGormHealth(t *testing.T) {
	ok, err := Open(fakeDialector{}, PoolConfig{PingTimeout: time.Second}, Options{})
	assert.Error(t, err).Nil("open")
	defer func() { _ = ok.Destroy() }()

	ind := NewGormHealth("gorm:fake:", "main", ok.DB)
	if ind.HealthName() != "gorm:fake:main" {
		t.Fatalf("indicator name: want gorm:fake:main, got %s", ind.HealthName())
	}
	if err := ind.CheckHealth(context.Background()); err != nil {
		t.Fatalf("healthy backend must check UP: %v", err)
	}

	// A dialector whose pool pings fine after open but whose ping fails later
	// (e.g. server gone away) is simulated by closing the pool.
	closed, err := Open(fakeDialector{}, PoolConfig{PingTimeout: time.Second}, Options{})
	assert.Error(t, err).Nil("open")
	if err := closed.Destroy(); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	ind2 := NewGormHealth("gorm:fake:", "closed", closed.DB)
	if err := ind2.CheckHealth(context.Background()); err == nil {
		t.Fatal("closed pool must check DOWN")
	}

	// Compile-time interface conformance for the indicator this package exports.
	var _ health.Indicator = ind
}
