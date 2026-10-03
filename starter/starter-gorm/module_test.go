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
	"strings"
	"testing"
	"time"

	"go-spring.org/cloud"
	"go-spring.org/stdlib/testing/assert"
	"gorm.io/gorm"
)

// The tests below drive [NewDB] — the construction half of an entry, shared by
// every dialect starter — over the same fake dialector the Open tests use. What
// a dialect starter owns (the gs.Module registration, the bean naming, the
// health indicator) is covered end to end by starter-gorm-sqlite, which opens a
// real in-memory database.

// specFor builds the minimal Spec a healthy fake entry needs.
func specFor(closers ...func()) Spec {
	return Spec{
		Dialector: fakeDialector{},
		Pool:      PoolConfig{PingTimeout: time.Second},
		Service:   "gorm:fake:life",
		Closers:   closers,
	}
}

// TestNewDBAssemblesWithoutProbe covers the default path (ping unset): the client
// is assembled complete — observe plugin and resilience callbacks installed — and
// no startup probe runs, so the wiring can skip a backend that is not up yet.
func TestNewDBAssemblesWithoutProbe(t *testing.T) {
	db, err := NewDB(context.Background(), "fake", specFor(), cloud.ClientParams{}, false)
	assert.Error(t, err).Nil("new db")
	defer func() { _ = db.Destroy() }()

	if err := Ping(context.Background(), db.DB); err != nil {
		t.Fatalf("assembled client must be usable: %v", err)
	}
}

// TestNewDBProbePasses proves the opt-in startup probe runs against a reachable
// backend without rejecting it.
func TestNewDBProbePasses(t *testing.T) {
	db, err := NewDB(context.Background(), "fake", specFor(), cloud.ClientParams{}, true)
	assert.Error(t, err).Nil("new db with ping")
	defer func() { _ = db.Destroy() }()

	if db == nil {
		t.Fatal("a healthy probe must still return the client")
	}
}

// TestNewDBReleasesClosersOnOpenFailure pins the rollback half: when the dialect's
// build succeeded (it already registered a discovery watch / TLS config) and the
// open then fails, NewDB releases exactly that driver-scoped state.
func TestNewDBReleasesClosersOnOpenFailure(t *testing.T) {
	closerRan := false
	spec := Spec{Dialector: fakeDialector{failInit: true}, Closers: []func(){func() { closerRan = true }}}

	db, err := NewDB(context.Background(), "fake", spec, cloud.ClientParams{}, false)
	if err == nil {
		_ = db.Destroy()
		t.Fatal("an initialize failure must fail the assembly")
	}
	if db != nil {
		t.Fatalf("expected nil client on failure, got %v", db)
	}
	if !closerRan {
		t.Fatal("a failed open must release the closers the dialect had armed")
	}
}

// TestNewDBProbeFailureAbandonsClient pins the probe's failure path: the probe is
// the last step, so a DB that fails it is destroyed (running the closers) and the
// error names the startup ping. The pool is closed by a customizer, which is the
// only way to make a live fake backend stop answering after a successful open.
func TestNewDBProbeFailureAbandonsClient(t *testing.T) {
	withCustomizers(t, func(db *gorm.DB) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	})

	closerRan := false
	db, err := NewDB(context.Background(), "fake", specFor(func() { closerRan = true }), cloud.ClientParams{}, true)
	if err == nil {
		_ = db.Destroy()
		t.Fatal("a failed startup probe must fail the assembly")
	}
	if db != nil {
		t.Fatalf("expected nil client on probe failure, got %v", db)
	}
	if !strings.Contains(err.Error(), "startup ping failed") {
		t.Fatalf("want the error to name the startup ping, got %v", err)
	}
	if !closerRan {
		t.Fatal("an abandoned client must release the closers the dialect had armed")
	}
}
