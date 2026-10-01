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

package gormobservability

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	gormresilience "go-spring.org/starter-gorm/resilience"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// The tests below compose the REAL stack: the observe plugin declares the
// operation on the call's context, then the resilience wrapper — the single
// emitter — reads it back. What is asserted here is the half this package owns:
// the declaration reaches the emitter with the identity gorm demands (name,
// db.system/db.operation, the SQL statement in Detail and the access tag). What
// the emitter builds from it is covered by resilience's own tests.

// probeConn is a minimal database/sql driver Conn: the tests run gorm in
// DryRun, so no statement ever reaches it — only the gorm.Open ping does.
type probeConn struct{}

func (probeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("probe: prepare unsupported")
}
func (probeConn) Close() error               { return nil }
func (probeConn) Begin() (driver.Tx, error)  { return nil, errors.New("probe: begin unsupported") }
func (probeConn) Ping(context.Context) error { return nil }

type probeConnector struct{}

func (probeConnector) Connect(context.Context) (driver.Conn, error) { return probeConn{}, nil }
func (probeConnector) Driver() driver.Driver                        { return probeDriver{} }

type probeDriver struct{}

func (probeDriver) Open(string) (driver.Conn, error) { return probeConn{}, nil }

// probeDialector hands gorm a *sql.DB over the probe driver and registers the
// default callbacks, so the plugin finds its gorm:create/query anchors.
type probeDialector struct{}

func (probeDialector) Name() string { return "probe" }

func (probeDialector) Initialize(db *gorm.DB) error {
	db.ConnPool = sql.OpenDB(probeConnector{})
	callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{})
	return nil
}

func (probeDialector) Migrator(*gorm.DB) gorm.Migrator { return nil }
func (probeDialector) DataTypeOf(*schema.Field) string { return "" }
func (probeDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return clause.Expr{}
}
func (probeDialector) BindVarTo(w clause.Writer, _ *gorm.Statement, _ any) { _, _ = w.WriteString("?") }
func (probeDialector) QuoteTo(w clause.Writer, s string)                   { _, _ = w.WriteString("`" + s + "`") }
func (probeDialector) Explain(s string, _ ...any) string                   { return s }

type probeUser struct {
	ID   uint
	Name string
}

func (probeUser) TableName() string { return "probe_users" }

// captureExecutor is the innermost layer the stack runs on: it records the
// Operation the declaration layer put on the context, then runs the call. The
// operation it sees is exactly what the real emitter (wrapped around it) would
// report.
type captureExecutor struct {
	op  observability.Operation
	has bool
}

func (c *captureExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	c.op, c.has = observability.OperationFrom(ctx)
	return fn(ctx)
}
func (c *captureExecutor) Close() error                          { return nil }
func (c *captureExecutor) Refresh(resilience.ClientPolicy) error { return nil }

// newProbeDB opens a gorm.DB with the plugin installed and the real resilience
// wrapper composed on top, returning the capture executor the wrapper forwards
// to. The wrapper is the same one gormcore.Open installs from
// cloud.ClientParams while assembling the client. The DB runs DryRun (with
// the default transaction off) so gorm builds every statement without touching
// the driver.
func newProbeDB(t *testing.T, system string) (*gorm.DB, *captureExecutor) {
	t.Helper()
	db, err := gorm.Open(probeDialector{}, &gorm.Config{DryRun: true, SkipDefaultTransaction: true})
	assert.Error(t, err).Nil("gorm open")
	if err := db.Use(NewPlugin(system)); err != nil {
		t.Fatalf("use plugin: %v", err)
	}
	inner := &captureExecutor{}
	exec := resilience.WrapClientExecutor(inner, "gorm", "svc")
	if err := gormresilience.ApplyCallbacks(db, exec, "svc"); err != nil {
		t.Fatalf("apply callbacks: %v", err)
	}
	return db, inner
}

// attrsMap flattens an attribute list into a map. Value.Emit, not AsString:
// AsString renders any non-STRING value as the empty string, which would make
// an assertion pass vacuously.
func attrsMap(kvs []attribute.KeyValue) map[string]string {
	m := make(map[string]string, len(kvs))
	for _, a := range kvs {
		m[string(a.Key)] = a.Value.Emit()
	}
	return m
}

// TestQueryDeclarationReachesEmitter is the core contract: the plugin declares
// the operation, and the resilience wrapper — the only emitter — receives it
// with the family's labels and the SQL builder's statement.
func TestQueryDeclarationReachesEmitter(t *testing.T) {
	db, inner := newProbeDB(t, "mysql")

	var users []probeUser
	tx := db.Model(&probeUser{}).Where("name = ?", "x").Find(&users)
	assert.Error(t, tx.Error).Nil("find")

	assert.That(t, inner.has).Equal(true)
	op := inner.op
	assert.That(t, op.Name).Equal("query")
	assert.That(t, op.Metric).Equal("db.client")
	// The access log keeps the tag it had before the move to a single emitter.
	assert.That(t, op.LogTag).Same(accessTag)

	attrs := attrsMap(op.Attrs)
	assert.That(t, attrs["db.system"]).Equal("mysql")
	assert.That(t, attrs["db.operation"]).Equal("query")

	// The statement is per-call detail: it must be present (what the SQL builder
	// produced) and must ride Detail, never Attrs — the whole point of the split,
	// since an unbounded statement as a metric label would explode the series.
	detail := attrsMap(op.Detail)
	assert.That(t, detail["db.statement"]).Equal("SELECT * FROM `probe_users` WHERE name = ?")
	assert.That(t, attrs["db.statement"]).Equal("")
}

// TestCreateDeclarationReachesEmitter covers the second processor kind, so the
// declaration is proven for both the query and the exec path.
func TestCreateDeclarationReachesEmitter(t *testing.T) {
	db, inner := newProbeDB(t, "postgresql")

	u := probeUser{ID: 1, Name: "x"}
	tx := db.Create(&u)
	assert.Error(t, tx.Error).Nil("create")

	assert.That(t, inner.has).Equal(true)
	assert.That(t, inner.op.Name).Equal("create")
	assert.That(t, attrsMap(inner.op.Attrs)["db.system"]).Equal("postgresql")
	if len(inner.op.Detail) == 0 {
		t.Fatal("expected the built INSERT to be declared as Detail")
	}
	if got := attrsMap(inner.op.Detail)["db.statement"]; !strings.Contains(got, "INSERT INTO `probe_users`") {
		t.Fatalf("expected the built INSERT, got %q", got)
	}
}

// TestStatementTruncation pins maxStatement: a runaway statement cannot flood
// the span or the log.
func TestStatementTruncation(t *testing.T) {
	db, inner := newProbeDB(t, "mysql")

	long := probeUser{Name: strings.Repeat("x", maxStatement*2)}
	tx := db.Create(&long)
	assert.Error(t, tx.Error).Nil("create")

	assert.That(t, inner.has).Equal(true)
	if len(inner.op.Detail) == 0 {
		t.Fatal("expected Detail")
	}
	if got := inner.op.Detail[0].Value.Emit(); len(got) > maxStatement {
		t.Fatalf("statement not truncated: %d bytes", len(got))
	}
}
