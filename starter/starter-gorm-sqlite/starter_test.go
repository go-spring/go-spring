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

package StarterGormSqlite

import (
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/spring/gs"
	gormcore "go-spring.org/starter-gorm"
)

// The glebarez driver is pure Go, so these tests run the real assembly chain —
// gs container → BindEach → build → gormcore.Open → Init (observe + resilience
// callbacks) → real SQL — against an in-memory SQLite database, no docker and
// no cgo required.

// kv is the toy model the lifecycle test round-trips through the DB.
type kv struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

// TestSqliteAssembly covers the full happy path: one configured instance opens
// a real in-memory database, the bound defaults hold, real SQL round-trips, the
// per-instance health indicator reports UP, and the wrapper queries work after
// Init installed the observe plugin + resilience callbacks.
func TestSqliteAssembly(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.gorm.sqlite.mem.file", ":memory:")
	}).RunTest(t, func(s *struct {
		DBs  []*DB              `autowire:""`
		Inds []*health.Indicator `autowire:""`
	}) {
		if len(s.DBs) != 1 {
			t.Fatalf("want 1 DB bean, got %d", len(s.DBs))
		}
		db := s.DBs[0]
		if err := gormcore.Ping(context.Background(), db.DB); err != nil {
			t.Fatalf("opened instance must ping: %v", err)
		}

		// Real SQL through the shared wrapper: migrate, insert, query back.
		if err := db.AutoMigrate(&kv{}); err != nil {
			t.Fatalf("automigrate: %v", err)
		}
		if err := db.Create(&kv{Key: "hello", Value: "world"}).Error; err != nil {
			t.Fatalf("create: %v", err)
		}
		var got kv
		if err := db.First(&got, "key = ?", "hello").Error; err != nil {
			t.Fatalf("query: %v", err)
		}
		if got.Value != "world" {
			t.Fatalf("round-trip mismatch: got %+v", got)
		}

		// One health indicator per instance, named after the instance.
		if len(s.Inds) != 1 || s.Inds[0].Name != "gorm:sqlite:mem" {
			t.Fatalf("want gorm:sqlite:mem indicator, got %+v", s.Inds)
		}
		if err := s.Inds[0].Probe(context.Background()); err != nil {
			t.Fatalf("indicator must report UP: %v", err)
		}
	})
}

// TestSqliteDefaultsNotTriggered proves the conditional wiring: with no
// spring.gorm.sqlite.* entries the starter registers nothing and the app starts.
func TestSqliteDefaultsNotTriggered(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		DBs  []*DB              `autowire:""`
		Inds []*health.Indicator `autowire:""`
	}) {
		if len(s.DBs) != 0 || len(s.Inds) != 0 {
			t.Fatalf("starter must stay dormant without config, got %d DB / %d indicators", len(s.DBs), len(s.Inds))
		}
	})
}

// TestBuildSpec pins the Spec the dialect hands to gormcore for this Config:
// the engine resource label and the observe flag flow through.
func TestBuildSpec(t *testing.T) {
	c := Config{File: ":memory:"}
	c.ObserveEnabled = false
	spec, err := build(context.Background(), c)
	assert.Error(t, err).Nil("build")
	if spec.Dialector == nil {
		t.Fatal("build must return a dialector")
	}
	if spec.Dialector.Name() != "sqlite" {
		t.Fatalf("dialector name: want sqlite, got %s", spec.Dialector.Name())
	}
	if spec.ObserveEnabled {
		t.Fatal("ObserveEnabled must flow from Config to Spec")
	}
}
