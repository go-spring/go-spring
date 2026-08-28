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

// query.go is the "command seam" concept of this starter: the guarded *Query
// wrapper every Client.Query/Client.Bind call returns. gocql exposes no
// reject-capable middleware (QueryObserver only watches), so the guard rides
// the query object itself — the statement-level analog of starter-tdengine's
// guardedConn. Coverage is transparent: an app that calls
// client.Query(...).Exec()/Iter()/Scan... gets the governance guard without
// any opt-in flag or special helper.
package StarterCassandra

import (
	"context"

	"github.com/gocql/gocql"
)

// Query wraps a *gocql.Query so its execution methods (Exec, Iter, Scan,
// ScanCAS, MapScan, MapScanCAS) route through the client's resilience
// executor + observer. All other gocql.Query methods (Consistency, PageSize,
// Idempotent, ...) promote unchanged through the embedding; note that the
// promoted configurators return the embedded *gocql.Query, so calling them
// after execution methods (or storing the result as a *gocql.Query) drops the
// guard — configure the wrapper's own methods (WithContext, Bind) to stay on
// the guarded path.
type Query struct {
	*gocql.Query
	// c is the owning client (executor + observer).
	c *Client
	// stmt is the CQL text, retained for the observation summary.
	stmt string
}

// Query shadows the embedded (*gocql.Session).Query: instead of a raw
// *gocql.Query it returns a guarded wrapper, so the normal statement path is
// transparently protected. It is the only signature change versus a raw
// session — see the type comment for the chaining caveat.
func (o *Client) Query(stmt string, values ...any) *Query {
	return &Query{Query: o.Session.Query(stmt, values...), c: o, stmt: stmt}
}

// Bind shadows the embedded (*gocql.Session).Bind for the same reason as
// Query: bound statements get the same transparent guard.
func (o *Client) Bind(stmt string, b func(q *gocql.QueryInfo) ([]any, error)) *Query {
	return &Query{Query: o.Session.Bind(stmt, b), c: o, stmt: stmt}
}

// WithContext attaches ctx to the query (the executor may derive a per-attempt
// timeout from it) and stays on the guarded wrapper.
func (q *Query) WithContext(ctx context.Context) *Query {
	q.Query = q.Query.WithContext(ctx)
	return q
}

// Bind shadows the embedded configurator so chaining stays on the guarded
// wrapper.
func (q *Query) Bind(v ...any) *Query {
	q.Query = q.Query.Bind(v...)
	return q
}

// Exec executes the statement synchronously through the guard. On rejection
// (rate-limit or open circuit) the statement is never attempted.
func (q *Query) Exec() error {
	return q.c.guard(q.context(), "exec", q.stmt, func(ctx context.Context) error {
		return q.Query.WithContext(ctx).Exec()
	})
}

// Iter executes the query and returns the first page's iterator through the
// guard. Flow control applies (a rate-limited or open-circuit call never
// reaches the cluster); the iterator's own error surfaces to the caller on
// Scan/Close as usual, because gocql offers no exported way to read it before
// then. Subsequent page fetches happen inside the returned *gocql.Iter and
// are not additionally guarded (the same statement-level fidelity as the
// database/sql starters).
func (q *Query) Iter() *gocql.Iter {
	var it *gocql.Iter
	_ = q.c.guard(q.context(), "query", q.stmt, func(ctx context.Context) error {
		it = q.Query.WithContext(ctx).Iter()
		return nil
	})
	return it
}

// Scan executes the query and scans the first row through the guard.
func (q *Query) Scan(dest ...any) error {
	return q.c.guard(q.context(), "query", q.stmt, func(ctx context.Context) error {
		return q.Query.WithContext(ctx).Scan(dest...)
	})
}

// ScanCAS executes a light-weight transaction and scans the first row through
// the guard.
func (q *Query) ScanCAS(dest ...any) (applied bool, err error) {
	err = q.c.guard(q.context(), "query", q.stmt, func(ctx context.Context) error {
		var e error
		applied, e = q.Query.WithContext(ctx).ScanCAS(dest...)
		return e
	})
	return applied, err
}

// MapScan executes the query and scans the first row into a map through the
// guard.
func (q *Query) MapScan(m map[string]any) error {
	return q.c.guard(q.context(), "query", q.stmt, func(ctx context.Context) error {
		return q.Query.WithContext(ctx).MapScan(m)
	})
}

// MapScanCAS executes a light-weight transaction and scans the first row into
// a map through the guard.
func (q *Query) MapScanCAS(m map[string]any) (applied bool, err error) {
	err = q.c.guard(q.context(), "query", q.stmt, func(ctx context.Context) error {
		var e error
		applied, e = q.Query.WithContext(ctx).MapScanCAS(m)
		return e
	})
	return applied, err
}

// context returns the query's context, defaulting to background when none was
// attached (mirroring gocql's own fallback).
func (q *Query) context() context.Context {
	if c := q.Query.Context(); c != nil {
		return c
	}
	return context.Background()
}
