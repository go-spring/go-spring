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
// interface + the bundled DefaultDriver, which owns full client assembly —
// parsing the DSN into a taosWS connector, wrapping it in the guarded
// connector/conn pair, and building the *sql.DB pool. It mirrors
// starter-gorm-mysql's driver shape.
package StarterTdengine

import (
	"context"
	"database/sql/driver"

	taosws "github.com/taosdata/driver-go/v3/taosWS"
	"go-spring.org/cloud"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a TDengine client (the starter's
// Client wrapper). It is an OPTIONAL CONTAINER BEAN: a company or umbrella
// starter may provide its own Driver bean (its constructor returns
// StarterTdengine.Driver); when none is present, starter-tdengine falls back to
// the bundled [DefaultDriver] inside client assembly. A custom driver is a
// bean, so it may inject the configuration/beans it needs — e.g. company config
// bound from a properties file at wiring time.
//
// CreateClient returns the module's exported [Client] — the wrapper apps inject
// — not the raw *sql.DB, so a driver takes part in the type the rest of the
// ecosystem sees. It returns the client COMPLETE: params supplies the
// container's facilities (see [cloud.ClientParams]), which [NewClient] applies
// while building. Nothing patches the client afterwards.
//
// params is one struct rather than a parameter per capability so this interface
// — which every company driver implements — stays stable as capabilities are
// added. A driver that has no use for one of its fields simply ignores it.
//
// At most one Driver bean is expected per process; every client under
// ${spring.tdengine} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new TDengine client from the provided configuration.
// It owns the driver-specific half of assembly — parsing the DSN into a taosWS
// connector — and hands the connector to [NewClient], which builds the guarded
// pool so the starter can route per-statement resilience + observability
// (database/sql offers no transport to swap, so the guard rides the driver.Conn
// level). params is passed straight through to [NewClient], so the client is
// assembled complete — identity and governance both applied — before it is
// returned. It does not run the startup ping probe, which is the starter's
// lifecycle concern (newClient in starter.go).
func (DefaultDriver) CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error) {
	cfg, err := taosws.ParseDSN(c.DSN)
	if err != nil {
		return nil, errutil.Explain(err, "tdengine: invalid dsn")
	}
	conn, err := taosws.NewConnector(cfg)
	if err != nil {
		return nil, errutil.Explain(err, "tdengine: connector failed")
	}
	// NewClient is the only way to build a Client: it fixes the identity,
	// installs the per-statement guard and applies governance (see [NewClient]),
	// so the client it returns is complete.
	return NewClient(conn, c, params), nil
}

// clientSlot carries the resilience executor [NewClient] installs. Connections
// consult it on every statement; until it is set it is transparent (nil exec).
type clientSlot struct {
	exec resilience.ClientExecutor
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

// guardedConn routes statements through the slot's executor when applied.
// Everything else delegates to the wrapped taosWS connection. This is the
// TDengine seam of resilience and observability: database/sql has no interceptor
// chain, so the guard lives at the driver.Conn level — the database/sql analog of
// the gorm callback chain and the http.RoundTripper adapters.
//
// The statement's semantic identity is declared on the context BEFORE it enters
// the executor (see [operation] and [observability.WithOperation]). The order
// matters: the executor reads the declaration at Execute entry, so a declaration
// made inside it — per attempt — would be read by nobody. The executor is also
// the one emitter of the span, the metrics and the access log; this layer only
// declares.
type guardedConn struct {
	base driver.Conn
	slot *clientSlot
}

// ExecContext declares the statement's identity, then runs it through the
// executor, which emits the call's signals. When no executor is applied the
// statement runs inline and the declaration is inert.
func (g guardedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	ctx = observability.WithOperation(ctx, operation("exec", query))
	var res driver.Result
	err := g.run(ctx, func(ctx context.Context) error {
		var err error
		res, err = execContext(g.base, ctx, query, args)
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// QueryContext declares the statement's identity, then runs it through the
// executor, which emits the call's signals. When no executor is applied the
// statement runs inline and the declaration is inert.
func (g guardedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	ctx = observability.WithOperation(ctx, operation("query", query))
	var rows driver.Rows
	err := g.run(ctx, func(ctx context.Context) error {
		var err error
		rows, err = queryContext(g.base, ctx, query, args)
		return err
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// run executes call under the slot's resilience executor, with the statement's
// identity already declared on ctx. [resilience.Run] is the shared seam: a
// protection rejection is returned verbatim, a fault injector's short-circuited
// error is preferred over a nil call error, and the operation's own error is
// returned otherwise. When no executor is applied, Run runs the call inline, so
// the declaration is carried but nothing emits — the zero-config pass-through.
func (g guardedConn) run(ctx context.Context, call func(context.Context) error) error {
	_, err := resilience.Run(ctx, g.slot.exec, func(attemptCtx context.Context) (struct{}, error) {
		return struct{}{}, call(attemptCtx)
	})
	return err
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
