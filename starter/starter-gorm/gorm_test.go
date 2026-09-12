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
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// The lifecycle tests below open a real gorm.DB through a hand-rolled fake
// dialector backed by a minimal database/sql driver (a Pinger-only Conn). That
// keeps this module dependency-free while exercising the full shared chain:
// gorm.Open → ApplyPool ping → DBCustomizers → wrapper → Init/Destroy.

// fakeConnector adapts the fake driver to database/sql.
type fakeConnector struct{ failPing bool }

func (c fakeConnector) Connect(context.Context) (driver.Conn, error) {
	return fakeConn{failPing: c.failPing}, nil
}
func (c fakeConnector) Driver() driver.Driver { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{}, nil }

// fakeConn is a minimal Conn whose Ping answers from the failPing flag; no
// statement work is ever routed through it in these tests.
type fakeConn struct{ failPing bool }

func (c fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake: prepare unsupported")
}
func (c fakeConn) Close() error              { return nil }
func (c fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("fake: begin unsupported") }
func (c fakeConn) Ping(context.Context) error {
	if c.failPing {
		return errors.New("fake: ping refused")
	}
	return nil
}

// fakeDialector hands gorm a *sql.DB over the fake driver and registers the
// default callbacks so Init's observe/resilience plugin registration finds its
// gorm:create/query/update/delete anchors.
type fakeDialector struct {
	failInit  bool
	failPing  bool
	closeSeen *bool
}

func (d fakeDialector) Name() string { return "fake" }

func (d fakeDialector) Initialize(db *gorm.DB) error {
	if d.failInit {
		return errors.New("fake: initialize refused")
	}
	db.ConnPool = sql.OpenDB(fakeConnector{failPing: d.failPing})
	callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{})
	return nil
}

func (d fakeDialector) Migrator(*gorm.DB) gorm.Migrator { return nil }
func (d fakeDialector) DataTypeOf(*schema.Field) string { return "" }
func (d fakeDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return clause.Expr{}
}
func (d fakeDialector) BindVarTo(w clause.Writer, _ *gorm.Statement, _ any) {
	_, _ = w.WriteString("?")
}
func (d fakeDialector) QuoteTo(clause.Writer, string)     {}
func (d fakeDialector) Explain(s string, _ ...any) string { return s }

// withCustomizers swaps the package-global customizer chain for the duration of
// one test, so a failing customizer registered here cannot leak into the other
// Open-based tests in this package.
func withCustomizers(t *testing.T, fs ...DBCustomizer) {
	t.Helper()
	old := customizers
	customizers = fs
	t.Cleanup(func() { customizers = old })
}

func TestCommonPool(t *testing.T) {
	c := Common{
		PoolSettings: PoolSettings{
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
			ConnMaxIdleTime: 30 * time.Minute,
			PingTimeout:     2 * time.Second,
			SlowThreshold:   100 * time.Millisecond,
		},
	}
	p := c.Pool()
	if p.MaxOpenConns != 10 || p.MaxIdleConns != 5 || p.ConnMaxLifetime != time.Hour ||
		p.ConnMaxIdleTime != 30*time.Minute || p.PingTimeout != 2*time.Second || p.SlowThreshold != 100*time.Millisecond {
		t.Fatalf("Pool() dropped fields: %+v", p)
	}
}

// TestNewResolverRequiresBackend pins the fail-loud rule the wiring relies on: a
// discovery-routed entry (service-name set) whose ${discovery} label named no
// bean must error instead of silently dialing the configured address. The
// backend is an argument now, so the rule is exercised by passing nil.
func TestNewResolverRequiresBackend(t *testing.T) {
	c := Common{ServiceName: "user-db", Discovery: "nope"}
	if _, err := c.NewResolver(context.Background(), nil); err == nil {
		t.Fatal("expected an error when service-name is set but the label named no backend")
	}
	// No service-name → discovery not in effect, no backend needed.
	if r, err := (Common{}).NewResolver(context.Background(), nil); err != nil || r != nil {
		t.Fatalf("expected (nil, nil) without service-name, got %v, %v", r, err)
	}
}

func TestGormConfig(t *testing.T) {
	// No slow threshold → gorm's default logger stays in place.
	if cfg := GormConfig(PoolConfig{}); cfg.Logger != nil {
		t.Fatal("expected nil logger when SlowThreshold is unset")
	}
	// Slow threshold set → a warn-level slow-query logger is installed.
	if cfg := GormConfig(PoolConfig{SlowThreshold: 200 * time.Millisecond}); cfg.Logger == nil {
		t.Fatal("expected a logger when SlowThreshold is set")
	}
}

func TestOpenLifecycle(t *testing.T) {
	closed := false
	closerRan := false

	var seen *gorm.DB
	withCustomizers(t, func(db *gorm.DB) error { seen = db; return nil })

	db, err := Open(fakeDialector{closeSeen: &closed}, PoolConfig{
		MaxOpenConns: 2,
		MaxIdleConns: 1,
		PingTimeout:  time.Second,
	}, Options{
		Engine:         "fake",
		Resource:       "gorm:fake:life",
		ObserveEnabled: true, // exercises the observe plugin + resilience callbacks path
		Closers:        []func(){func() { closerRan = true }},
	})
	assert.Error(t, err).Nil("open")
	defer func() { _ = db.Destroy() }()

	if seen == nil || seen != db.DB {
		t.Fatal("customizer should run after open with the freshly-opened *gorm.DB")
	}

	// Init installs the observe plugin and resilience callbacks; with
	// governance off the executor is a transparent no-op, so this must succeed.
	if err := db.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := Ping(context.Background(), db.DB); err != nil {
		t.Fatalf("ping after init: %v", err)
	}
	if _, err := Stats(db.DB); err != nil {
		t.Fatalf("stats: %v", err)
	}

	if err := db.Destroy(); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if !closerRan {
		t.Fatal("destroy should run the registered closer")
	}
	// Ping after close proves the underlying pool was actually closed.
	if err := Ping(context.Background(), db.DB); err == nil {
		t.Fatal("expected ping to fail after Destroy closed the pool")
	}
}

func TestOpenDialectorFailure(t *testing.T) {
	_, err := Open(fakeDialector{failInit: true}, PoolConfig{}, Options{})
	if err == nil || !strings.Contains(err.Error(), "gorm open") {
		t.Fatalf("want gorm open error, got %v", err)
	}
}

func TestOpenPingFailure(t *testing.T) {
	withCustomizers(t) // none
	// gorm.Open auto-pings on Initialize, so a failing backend surfaces as the
	// open error here (ApplyPool's ping would catch it otherwise); either way
	// the starter must fail fast instead of returning an unusable DB.
	_, err := Open(fakeDialector{failPing: true}, PoolConfig{PingTimeout: 500 * time.Millisecond}, Options{})
	if err == nil || !strings.Contains(err.Error(), "ping refused") {
		t.Fatalf("want ping failure to fail the open, got %v", err)
	}
}

func TestOpenCustomizerFailureClosesPool(t *testing.T) {
	withCustomizers(t, func(*gorm.DB) error { return errors.New("boom") })
	db, err := Open(fakeDialector{}, PoolConfig{PingTimeout: time.Second}, Options{})
	if err == nil || !strings.Contains(err.Error(), "gorm customizer") {
		t.Fatalf("want gorm customizer error, got %v", err)
	}
	if db != nil {
		t.Fatal("expected nil DB on customizer failure")
	}
}

func TestDBCustomizerChain(t *testing.T) {
	withCustomizers(t)

	// nil customizers are ignored by registration.
	UseDBCustomizer(nil)
	if len(customizers) != 0 {
		t.Fatalf("nil customizer must not register, have %d", len(customizers))
	}

	// The chain runs in registration order; the first error stops it.
	var order []string
	UseDBCustomizer(func(*gorm.DB) error { order = append(order, "a"); return nil })
	UseDBCustomizer(func(*gorm.DB) error { order = append(order, "b"); return errors.New("stop") })
	UseDBCustomizer(func(*gorm.DB) error { order = append(order, "c"); return nil })

	if err := ApplyDBCustomizers(nil); err == nil {
		t.Fatal("expected the chain to propagate the customizer error")
	}
	if strings.Join(order, ",") != "a,b" {
		t.Fatalf("chain must stop at first error, got %v", order)
	}
}
