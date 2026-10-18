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
// builder every Client.Query/Client.Bind call returns, hollowed into a
// per-query chain. gocql exposes no reject-capable middleware (QueryObserver
// only watches), so the chain is the only seam: the guarded layer builds each
// terminal call's statement identity and runs it under the resilience executor
// (the operation is passed explicitly to the executor, not carried on the
// context), and the adapter layer executes the builder's current *gocql.Query.
// A custom layer may wrap the chain head to modify what a statement does.

package StarterCassandra

import (
	"context"

	"github.com/gocql/gocql"
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
)

// InnerQuery is the terminal-execution seam a statement's traffic runs
// through, one chain per query. gocql offers no hook or plugin point, so this
// interface is the ONLY way to modify what happens under the Query builder's
// Exec/Iter/Scan-family methods.
//
// The default chain is the guarded layer over a raw adapter, and the builder's
// embedded InnerQuery is where a custom layer goes: implement this interface
// (embed the head you found to inherit the methods you do not care about), then
// assign your layer over it. The chain under the layer keeps doing its job —
// the executor still protects every call.
//
// Release follows the chain protocol: every layer takes away its OWN
// resources and passes the flag to the layer under it. A query holds no
// resource, so every layer's Release is a pass-through — the instance the
// chain bottoms out in is the SESSION, released by [Client.Close].
type InnerQuery interface {
	// Exec executes the statement synchronously.
	Exec(ctx context.Context) error
	// Iter executes the query and returns the first page's iterator.
	Iter(ctx context.Context) *gocql.Iter
	// Scan executes the query and scans the first row.
	Scan(ctx context.Context, dest ...any) error
	// ScanCAS executes a light-weight transaction and scans the first row.
	ScanCAS(ctx context.Context, dest ...any) (bool, error)
	// MapScan executes the query and scans the first row into a map.
	MapScan(ctx context.Context, m map[string]any) error
	// MapScanCAS executes a light-weight transaction and scans the first row
	// into a map.
	MapScanCAS(ctx context.Context, m map[string]any) (bool, error)
	// Release releases the layer's own resources, then hands releaseRaw to
	// the layer under it.
	Release(releaseRaw bool) error
}

// RawQuery is the adapter layer at the tail of a query's chain: it reads the
// builder's CURRENT *gocql.Query (WithContext replaces the embedded object, so
// a pointer captured at construction would go stale) and executes the terminal
// call with the context the layers above derived. It holds no resource of its
// own — see [InnerQuery] for why its Release is a pass-through.
type RawQuery struct {
	q *Query
}

// NewRawQuery wraps a query builder as the [InnerQuery] tail of its chain.
func NewRawQuery(q *Query) *RawQuery { return &RawQuery{q: q} }

// Release is the protocol's pass-through: a query holds no resource; the
// session — the instance every query bottoms out in — is released by
// [Client.Close].
func (r *RawQuery) Release(bool) error { return nil }

func (r *RawQuery) Exec(ctx context.Context) error {
	return r.q.Query.WithContext(ctx).Exec()
}

func (r *RawQuery) Iter(ctx context.Context) *gocql.Iter {
	return r.q.Query.WithContext(ctx).Iter()
}

func (r *RawQuery) Scan(ctx context.Context, dest ...any) error {
	return r.q.Query.WithContext(ctx).Scan(dest...)
}

func (r *RawQuery) ScanCAS(ctx context.Context, dest ...any) (bool, error) {
	return r.q.Query.WithContext(ctx).ScanCAS(dest...)
}

func (r *RawQuery) MapScan(ctx context.Context, m map[string]any) error {
	return r.q.Query.WithContext(ctx).MapScan(m)
}

func (r *RawQuery) MapScanCAS(ctx context.Context, m map[string]any) (bool, error) {
	return r.q.Query.WithContext(ctx).MapScanCAS(m)
}

// GuardQuery is the guarded layer: it builds each terminal call's semantic
// identity — op ("exec" for Exec, "query" for the read family) and stmt, the
// statement — and runs the call under the resilience executor, which applies
// rate limiting, breaking and bulkheading — and emits the statement's span,
// metrics and access log from the one point that sees the whole call, retries
// included. The identity is built here and passed EXPLICITLY to the executor
// rather than carried on the context. One instance is built per client in
// [NewClient] and shared by every query's chain; its executor's lifetime is the
// session's, so the executor is closed by [Client.Close], not here.
// [NewGuardQuery] builds it.
type GuardQuery struct {
	exec    chain.Executor
	service string // the governance label the executor was built for
	next    InnerQuery
	stmt    string
}

// NewGuardQuery builds the guarded layer over next, supplying stmt as the
// statement's identity and running every terminal call under exec. service is
// the label exec was built for; each call states it on its operation so a layer
// outside exec (the fault layer) matches the same service.
func NewGuardQuery(next InnerQuery, exec chain.Executor, service, stmt string) *GuardQuery {
	return &GuardQuery{exec: exec, service: service, next: next, stmt: stmt}
}

// op builds one call's identity with the service label, the single place the
// label is attached.
func (g *GuardQuery) op(kind string) chain.Operation {
	o := operation(kind, g.stmt)
	o.Service = g.service
	return o
}

// Release hands releaseRaw to the layer under it — the executor is shared by
// every query of the client and closed with the session (see [Client.Close]),
// so this layer has nothing of its own to take away.
func (g *GuardQuery) Release(releaseRaw bool) error { return g.next.Release(releaseRaw) }

func (g *GuardQuery) Exec(ctx context.Context) error {
	return g.exec.Execute(ctx, g.op("exec"), nil, func(context.Context) error { return g.next.Exec(ctx) })
}

func (g *GuardQuery) Iter(ctx context.Context) *gocql.Iter {
	var it *gocql.Iter
	_ = g.exec.Execute(ctx, g.op("query"), nil, func(context.Context) error {
		it = g.next.Iter(ctx)
		return nil
	})
	return it
}

func (g *GuardQuery) Scan(ctx context.Context, dest ...any) error {
	return g.exec.Execute(ctx, g.op("query"), nil, func(context.Context) error { return g.next.Scan(ctx, dest...) })
}

func (g *GuardQuery) ScanCAS(ctx context.Context, dest ...any) (bool, error) {
	var applied bool
	err := g.exec.Execute(ctx, g.op("query"), nil, func(context.Context) error {
		var e error
		applied, e = g.next.ScanCAS(ctx, dest...)
		return e
	})
	return applied, err
}

func (g *GuardQuery) MapScan(ctx context.Context, m map[string]any) error {
	return g.exec.Execute(ctx, g.op("query"), nil, func(context.Context) error { return g.next.MapScan(ctx, m) })
}

func (g *GuardQuery) MapScanCAS(ctx context.Context, m map[string]any) (bool, error) {
	var applied bool
	err := g.exec.Execute(ctx, g.op("query"), nil, func(context.Context) error {
		var e error
		applied, e = g.next.MapScanCAS(ctx, m)
		return e
	})
	return applied, err
}

// Query wraps a *gocql.Query so its execution methods (Exec, Iter, Scan,
// ScanCAS, MapScan, MapScanCAS) run through the per-query [InnerQuery] chain
// instead of the raw statement. All other gocql.Query methods (Consistency,
// PageSize, Idempotent, ...) promote unchanged through the embedding; note
// that the promoted configurators return the embedded *gocql.Query, so calling
// them after execution methods (or storing the result as a *gocql.Query)
// drops the chain — configure the wrapper's own methods (WithContext, Bind) to
// stay on the guarded path.
type Query struct {
	*gocql.Query
	// InnerQuery is the chain this statement's terminal calls run through;
	// reorganize it by wrapping the head in a layer of your own (see
	// [InnerQuery]). Build-time only — write it before the statement executes.
	InnerQuery
	// stmt is the CQL text, retained for the identity declaration.
	stmt string
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

// Exec executes the statement synchronously through the chain. On rejection
// (rate-limit or open circuit) the statement is never attempted.
func (q *Query) Exec() error { return q.InnerQuery.Exec(q.context()) }

// Iter executes the query and returns the first page's iterator through the
// chain. Flow control applies (a rate-limited or open-circuit call never
// reaches the cluster); the iterator's own error surfaces to the caller on
// Scan/Close as usual, because gocql offers no exported way to read it before
// then. Subsequent page fetches happen inside the returned *gocql.Iter and
// are not additionally guarded (the same statement-level fidelity as the
// database/sql starters).
func (q *Query) Iter() *gocql.Iter { return q.InnerQuery.Iter(q.context()) }

// Scan executes the query and scans the first row through the chain.
func (q *Query) Scan(dest ...any) error { return q.InnerQuery.Scan(q.context(), dest...) }

// ScanCAS executes a light-weight transaction and scans the first row through
// the chain.
func (q *Query) ScanCAS(dest ...any) (bool, error) {
	return q.InnerQuery.ScanCAS(q.context(), dest...)
}

// MapScan executes the query and scans the first row into a map through the
// chain.
func (q *Query) MapScan(m map[string]any) error { return q.InnerQuery.MapScan(q.context(), m) }

// MapScanCAS executes a light-weight transaction and scans the first row into
// a map through the chain.
func (q *Query) MapScanCAS(m map[string]any) (bool, error) {
	return q.InnerQuery.MapScanCAS(q.context(), m)
}

// context returns the query's context, defaulting to background when none was
// attached (mirroring gocql's own fallback).
func (q *Query) context() context.Context {
	if c := q.Query.Context(); c != nil {
		return c
	}
	return context.Background()
}
