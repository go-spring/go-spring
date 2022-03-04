/*
 * Copyright 2012-2019 the original author or authors.
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

//go:generate mockgen -build_flags="-mod=mod" -package=driver -source=driver.go -destination=mock.go

package driver

import (
	"context"
	"database/sql/driver"
	"errors"
)

type Driver interface {
	Open(name string) (driver.Conn, error)
	OpenConnector(name string) (driver.Connector, error)
}

type Connector interface {
	Connect(context.Context) (driver.Conn, error)
	Driver() driver.Driver
}

type Conn interface {
	Ping(ctx context.Context) error
	// Deprecated: use PrepareContext instead.
	Prepare(query string) (driver.Stmt, error)
	Close() error
	// Deprecated: use BeginTx instead.
	Begin() (driver.Tx, error)
	BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error)
	PrepareContext(ctx context.Context, query string) (driver.Stmt, error)
	ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error)
	QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error)
	CheckNamedValue(*driver.NamedValue) error
	ResetSession(ctx context.Context) error
}

type Stmt interface {
	Close() error
	NumInput() int
	// Deprecated: use ExecContext instead.
	Exec(args []driver.Value) (driver.Result, error)
	// Deprecated: use QueryContext instead.
	Query(args []driver.Value) (driver.Rows, error)
	ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error)
	QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error)
	CheckNamedValue(*driver.NamedValue) error
}

type Result interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

type Rows interface {
	Columns() []string
	Close() error
	Next(dest []driver.Value) error
}

type NamedValue = driver.NamedValue

type (
	ConnPrepareContextFunc func(ctx context.Context, query string) (Stmt, error)
	ConnExecContextFunc    func(ctx context.Context, query string, args []driver.NamedValue) (Result, error)
	ConnQueryContextFunc   func(ctx context.Context, query string, args []driver.NamedValue) (Rows, error)
	StmtExecContextFunc    func(ctx context.Context, query string, args []driver.NamedValue) (Result, error)
	StmtQueryContextFunc   func(ctx context.Context, query string, args []driver.NamedValue) (Rows, error)
)

type Hook interface {
	ConnPrepareContext(ctx context.Context, query string, fn ConnPrepareContextFunc) (Stmt, error)
	ConnExecContext(ctx context.Context, query string, args []driver.NamedValue, fn ConnExecContextFunc) (Result, error)
	ConnQueryContext(ctx context.Context, query string, args []driver.NamedValue, fn ConnQueryContextFunc) (Rows, error)
	StmtExecContext(ctx context.Context, query string, args []driver.NamedValue, fn StmtExecContextFunc) (Result, error)
	StmtQueryContext(ctx context.Context, query string, args []driver.NamedValue, fn StmtQueryContextFunc) (Rows, error)
}

var config struct {
	hook Hook
}

// SetHook 设置事件钩子。
func SetHook(hook Hook) {
	config.hook = hook
}

type myDriver struct {
	driver Driver
}

func NewDriver(d driver.Driver) (Driver, error) {
	if v, ok := d.(Driver); ok {
		return &myDriver{driver: v}, nil
	}
	return nil, errors.New("should implement `Driver` interface")
}

func (d *myDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.driver.Open(name)
	if err != nil {
		return nil, err
	}
	return NewConn(conn)
}

func (d *myDriver) OpenConnector(name string) (driver.Connector, error) {
	connector, err := d.driver.OpenConnector(name)
	if err != nil {
		return nil, err
	}
	return &myConnector{connector: connector}, nil
}

type myConnector struct {
	connector driver.Connector
}

func (c *myConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return NewConn(conn)
}

func (c *myConnector) Driver() driver.Driver {
	return c.connector.Driver()
}

type myConn struct {
	conn Conn
}

func NewConn(conn driver.Conn) (driver.Conn, error) {
	if v, ok := conn.(Conn); ok {
		return &myConn{conn: v}, nil
	}
	return nil, errors.New("should implement `Conn` interface")
}

func (c *myConn) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

func (c *myConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("wouldn't call this method")
}

func (c *myConn) Close() error {
	return c.conn.Close()
}

func (c *myConn) Begin() (driver.Tx, error) {
	return nil, errors.New("wouldn't call this method")
}

func (c *myConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return c.conn.BeginTx(ctx, opts)
}

func (c *myConn) prepare(ctx context.Context, query string) (Stmt, error) {
	stmt, err := c.conn.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return newStmt(stmt, query)
}

func (c *myConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if config.hook != nil {
		return config.hook.ConnPrepareContext(ctx, query,
			func(ctx context.Context, query string) (Stmt, error) {
				return c.prepare(ctx, query)
			})
	}
	return c.prepare(ctx, query)
}

func (c *myConn) exec(ctx context.Context, query string, args []driver.NamedValue) (Result, error) {
	result, err := c.conn.ExecContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return newResult(result), nil
}

func (c *myConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if config.hook != nil {
		return config.hook.ConnExecContext(ctx, query, args,
			func(ctx context.Context, query string, args []driver.NamedValue) (Result, error) {
				return c.exec(ctx, query, args)
			})
	}
	return c.exec(ctx, query, args)
}

func (c *myConn) query(ctx context.Context, query string, args []driver.NamedValue) (Rows, error) {
	rows, err := c.conn.QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return newRows(rows), nil
}

func (c *myConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if config.hook != nil {
		return config.hook.ConnQueryContext(ctx, query, args,
			func(ctx context.Context, query string, args []driver.NamedValue) (Rows, error) {
				return c.query(ctx, query, args)
			})
	}
	return c.query(ctx, query, args)
}

func (c *myConn) CheckNamedValue(nv *driver.NamedValue) error {
	return c.conn.CheckNamedValue(nv)
}

func (c *myConn) ResetSession(ctx context.Context) error {
	return c.conn.ResetSession(ctx)
}

type myStmt struct {
	stmt Stmt
	sql  string
}

func newStmt(stmt driver.Stmt, sql string) (Stmt, error) {
	if _, ok := stmt.(Stmt); ok {
		return &myStmt{stmt: stmt.(Stmt), sql: sql}, nil
	}
	return nil, errors.New("should implement `Stmt` interface")
}

func (stmt *myStmt) Close() error {
	return stmt.stmt.Close()
}

func (stmt *myStmt) NumInput() int {
	return stmt.stmt.NumInput()
}

func (stmt *myStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("wouldn't call this method")
}

func (stmt *myStmt) exec(ctx context.Context, args []driver.NamedValue) (Result, error) {
	result, err := stmt.stmt.ExecContext(ctx, args)
	if err != nil {
		return nil, err
	}
	return newResult(result), nil
}

func (stmt *myStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if config.hook != nil {
		return config.hook.StmtExecContext(ctx, stmt.sql, args,
			func(ctx context.Context, query string, args []driver.NamedValue) (Result, error) {
				return stmt.exec(ctx, args)
			})
	}
	return stmt.exec(ctx, args)
}

func (stmt *myStmt) Query([]driver.Value) (driver.Rows, error) {
	return nil, errors.New("wouldn't call this method")
}

func (stmt *myStmt) query(ctx context.Context, args []driver.NamedValue) (Rows, error) {
	rows, err := stmt.stmt.QueryContext(ctx, args)
	if err != nil {
		return nil, err
	}
	return newRows(rows), nil
}

func (stmt *myStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if config.hook != nil {
		return config.hook.StmtQueryContext(ctx, stmt.sql, args,
			func(ctx context.Context, query string, args []driver.NamedValue) (Rows, error) {
				return stmt.query(ctx, args)
			})
	}
	return stmt.query(ctx, args)
}

func (stmt *myStmt) CheckNamedValue(nv *driver.NamedValue) error {
	return stmt.stmt.CheckNamedValue(nv)
}

type myResult struct {
	result driver.Result
}

func newResult(result driver.Result) driver.Result {
	return &myResult{result: result}
}

func (r *myResult) LastInsertId() (int64, error) {
	return r.result.LastInsertId()
}

func (r *myResult) RowsAffected() (int64, error) {
	return r.result.RowsAffected()
}

type myRows struct {
	rows driver.Rows
}

func newRows(rows driver.Rows) driver.Rows {
	return &myRows{rows: rows}
}

func (r *myRows) Columns() []string {
	return r.rows.Columns()
}

func (r *myRows) Close() error {
	return r.rows.Close()
}

func (r *myRows) Next(dest []driver.Value) error {
	return r.rows.Next(dest)
}
