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

package StarterTdengine

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// stubConn is a driver.Conn whose Exec/Query results are scripted.
type stubConn struct {
	execErr  error
	lastStmt string
}

func (c *stubConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (c *stubConn) Close() error                        { return nil }
func (c *stubConn) Begin() (driver.Tx, error)           { return nil, errors.New("no transactions") }

func (c *stubConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.lastStmt = query
	return nil, c.execErr
}

func (c *stubConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.lastStmt = query
	return nil, nil
}

// TestGuardedConnPassThrough proves the unarmed slot runs statements inline:
// the guard is a zero-config opt-in, matching the other client adapters.
func TestGuardedConnPassThrough(t *testing.T) {
	sc := &stubConn{}
	conn := guardedConn{base: sc, slot: &clientSlot{}}

	_, err := conn.ExecContext(context.Background(), "CREATE DATABASE x", nil)
	assert.Error(t, err).Nil()
	assert.That(t, sc.lastStmt).Equal("CREATE DATABASE x")
}

// TestGuardedConnRateLimit confirms the flow-control path: once the burst is
// spent, the statement is rejected without reaching the connection.
func TestGuardedConnRateLimit(t *testing.T) {
	d := resilience.NewDefaultDriver()
	exec, err := d.NewExecutor(resilience.Policy{RateLimit: 1, Burst: 1})
	assert.Error(t, err).Nil()

	sc := &stubConn{}
	conn := guardedConn{base: sc, slot: &clientSlot{exec: exec, resource: "tdengine:test"}}

	_, err = conn.ExecContext(context.Background(), "INSERT INTO t VALUES(now, 1)", nil)
	assert.Error(t, err).Nil()
	_, err = conn.ExecContext(context.Background(), "INSERT INTO t VALUES(now, 2)", nil)
	assert.Error(t, err).Is(resilience.ErrRateLimited)
	assert.That(t, sc.lastStmt).Equal("INSERT INTO t VALUES(now, 1)") // the rejected call never reached the conn
}

// TestDsnAddr covers the display-safe address extraction.
func TestDsnAddr(t *testing.T) {
	assert.That(t, dsnAddr("root:taosdata@ws(127.0.0.1:6041)/power")).Equal("127.0.0.1:6041")
	assert.That(t, dsnAddr("root:taosdata@wss(db.internal:6041)/power")).Equal("db.internal:6041")
	assert.That(t, dsnAddr("no-parens")).Equal("no-parens")
}
