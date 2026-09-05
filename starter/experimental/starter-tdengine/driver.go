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

// driver.go is the "construction seam" concept of this starter: the Driver
// interface + registry + DefaultDriver, which owns full client assembly —
// parsing the DSN into a taosWS connector, wrapping it in the guarded
// connector/conn pair, and building the *sql.DB pool. It mirrors
// starter-gorm-mysql's driver shape and starter-s3's registry.
package StarterTdengine

import (
	"context"
	"database/sql"
	"database/sql/driver"

	taosws "github.com/taosdata/driver-go/v3/taosWS"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/errutil"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Driver interface defines how to create a TDengine client (the starter's
// Client wrapper). It is the single extension point for customizing client
// assembly: an app (or the bundled DefaultDriver) implements it once and
// registers via RegisterDriver; callers select one through Config.Driver,
// which defaults to "DefaultDriver".
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*Client, error)
}

// RegisterDriver registers a TDengine driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("tdengine driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new TDengine client from the provided configuration.
// It owns full client assembly: parsing the DSN into a taosWS connector,
// wrapping it so Init can later arm per-statement resilience + observability
// (database/sql offers no transport to swap, so the guard rides the
// driver.Conn level), and applying the pool settings — but not the startup
// ping probe or the resilience wiring, which are the starter's lifecycle
// concerns (see newClient in starter.go).
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*Client, error) {
	cfg, err := taosws.ParseDSN(c.DSN)
	if err != nil {
		return nil, errutil.Explain(err, "tdengine: invalid dsn")
	}
	conn, err := taosws.NewConnector(cfg)
	if err != nil {
		return nil, errutil.Explain(err, "tdengine: connector failed")
	}

	slot := &clientSlot{}
	db := sql.OpenDB(guardedConnector{base: conn, slot: slot})
	db.SetMaxOpenConns(c.MaxOpenConns)
	db.SetMaxIdleConns(c.MaxIdleConns)
	db.SetConnMaxLifetime(c.ConnMaxLifetime)
	return &Client{DB: db, cfg: c, slot: slot}, nil
}

// clientSlot carries the executor + observer Init arms. Connections consult it
// on every statement; before Init it is transparent (nil exec, nil obs).
type clientSlot struct {
	exec     resilience.Executor
	resource string
	obs      *dbObserver
}

// guardedConnector wraps a driver.Connector so every connection it hands out
// is a guardedConn.
type guardedConnector struct {
	base driver.Connector
	slot *clientSlot
}

// Connect returns a guarded connection.
func (c guardedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	raw, err := c.base.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return guardedConn{base: raw, slot: c.slot}, nil
}

// Driver returns the wrapped connector's driver.
func (c guardedConnector) Driver() driver.Driver { return c.base.Driver() }

// guardedConn routes statements through the slot's executor (when armed) and
// observes each one through the slot's observer. Everything else delegates to
// the wrapped taosWS connection. This is the TDengine seam of resilience and
// observability: database/sql has no interceptor chain, so the guard lives at
// the driver.Conn level — the database/sql analog of the gorm callback chain
// and the http.RoundTripper adapters.
type guardedConn struct {
	base driver.Conn
	slot *clientSlot
}

// ExecContext runs the statement through the executor and observer when armed.
func (g guardedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	var res driver.Result
	call := func(ctx context.Context) error {
		var err error
		res, err = g.execObserved(ctx, query, func(ctx context.Context) (driver.Result, error) {
			return execContext(g.base, ctx, query, args)
		})
		return err
	}
	if err := g.guard(ctx, call); err != nil {
		return nil, err
	}
	return res, nil
}

// QueryContext runs the query through the executor and observer when armed.
func (g guardedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	var rows driver.Rows
	call := func(ctx context.Context) error {
		var err error
		rows, err = g.queryObserved(ctx, query, func(ctx context.Context) (driver.Rows, error) {
			return queryContext(g.base, ctx, query, args)
		})
		return err
	}
	if err := g.guard(ctx, call); err != nil {
		return nil, err
	}
	return rows, nil
}

// guard routes call through the slot's executor when one is armed, and
// otherwise runs it inline.
func (g guardedConn) guard(ctx context.Context, call func(context.Context) error) error {
	if g.slot.exec == nil {
		return call(ctx)
	}
	return g.slot.exec.Execute(ctx, g.slot.resource, call)
}

// execObserved opens an observation around the call when an observer is armed.
func (g guardedConn) execObserved(ctx context.Context, query string, call func(context.Context) (driver.Result, error)) (driver.Result, error) {
	if g.slot.obs == nil {
		return call(ctx)
	}
	ctx, sp := g.slot.obs.Start(ctx, "exec", query)
	res, err := call(ctx)
	sp.End(err)
	return res, err
}

// queryObserved opens an observation around the call when an observer is armed.
func (g guardedConn) queryObserved(ctx context.Context, query string, call func(context.Context) (driver.Rows, error)) (driver.Rows, error) {
	if g.slot.obs == nil {
		return call(ctx)
	}
	ctx, sp := g.slot.obs.Start(ctx, "query", query)
	rows, err := call(ctx)
	sp.End(err)
	return rows, err
}

// Prepare delegates to the wrapped connection (statement-level guards are not
// modeled; use ExecContext/QueryContext, which database/sql prefers anyway).
func (g guardedConn) Prepare(query string) (driver.Stmt, error) { return g.base.Prepare(query) }

// Close delegates to the wrapped connection.
func (g guardedConn) Close() error { return g.base.Close() }

// Begin delegates to the wrapped connection (TDengine has no transactions;
// the underlying Begin reports that).
func (g guardedConn) Begin() (driver.Tx, error) { return g.base.Begin() }

// execContext adapts a driver.Conn that implements driver.ExecerContext.
func execContext(c driver.Conn, ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	e, ok := c.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return e.ExecContext(ctx, query, args)
}

// queryContext adapts a driver.Conn that implements driver.QueryerContext.
func queryContext(c driver.Conn, ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q, ok := c.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return q.QueryContext(ctx, query, args)
}
