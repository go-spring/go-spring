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
// only watches), so the chain is the only seam: the identity layer declares
// each terminal call's statement, the governance layer runs it under the
// resilience executor, and the adapter layer executes the builder's current
// *gocql.Query. A custom layer may wrap the chain head to modify what a
// statement does — the rewrites are what the identity layer declares.
package StarterCassandra

import (
	"context"

	"github.com/gocql/gocql"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/observability"
)

// InnerQuery is the terminal-execution seam a statement's traffic runs
// through, one chain per query. gocql offers no hook or plugin point, so this
// interface is the ONLY way to modify what happens under the Query builder's
// Exec/Iter/Scan-family methods.
//
// The default chain is the identity layer over the governance layer over a raw
// adapter, and the builder's embedded InnerQuery is where a custom layer goes:
// implement this interface (embed the head you found to inherit the methods
// you do not care about), then assign your layer over it. The chain under the
// layer keeps doing its job — the statements the layer rewrites are what the
// identity layer declares, and the executor still protects every call.
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

// GuardQuery is the governance layer: it runs every terminal call under the
// resilience executor, which applies rate limiting, breaking and bulkheading —
// and emits the statement's span, metrics and access log from the one point
// that sees the whole call, retries included. One instance is built per client
// in [NewClient] and shared by every query's chain; its executor's lifetime is
// the session's, so the executor is closed by [Client.Close], not here.
// [NewGuardQuery] builds it.
type GuardQuery struct {
	exec chain.Executor
	next InnerQuery
}

// NewGuardQuery builds the governance layer over next, running every terminal
// call under exec.
func NewGuardQuery(next InnerQuery, exec chain.Executor) *GuardQuery {
	return &GuardQuery{exec: exec, next: next}
}

// Release hands releaseRaw to the layer under it — the executor is shared by
// every query of the client and closed with the session (see [Client.Close]),
// so this layer has nothing of its own to take away.
func (g *GuardQuery) Release(releaseRaw bool) error { return g.next.Release(releaseRaw) }

func (g *GuardQuery) Exec(ctx context.Context) error {
	return g.exec.Execute(ctx, func(context.Context) error { return g.next.Exec(ctx) })
}

func (g *GuardQuery) Iter(ctx context.Context) *gocql.Iter {
	var it *gocql.Iter
	_ = g.exec.Execute(ctx, func(context.Context) error {
		it = g.next.Iter(ctx)
		return nil
	})
	return it
}

func (g *GuardQuery) Scan(ctx context.Context, dest ...any) error {
	return g.exec.Execute(ctx, func(context.Context) error { return g.next.Scan(ctx, dest...) })
}

func (g *GuardQuery) ScanCAS(ctx context.Context, dest ...any) (bool, error) {
	var applied bool
	err := g.exec.Execute(ctx, func(context.Context) error {
		var e error
		applied, e = g.next.ScanCAS(ctx, dest...)
		return e
	})
	return applied, err
}

func (g *GuardQuery) MapScan(ctx context.Context, m map[string]any) error {
	return g.exec.Execute(ctx, func(context.Context) error { return g.next.MapScan(ctx, m) })
}

func (g *GuardQuery) MapScanCAS(ctx context.Context, m map[string]any) (bool, error) {
	var applied bool
	err := g.exec.Execute(ctx, func(context.Context) error {
		var e error
		applied, e = g.next.MapScanCAS(ctx, m)
		return e
	})
	return applied, err
}

// ObsQuery is the identity layer at the head of a query's chain: it names each
// terminal call — op ("exec" for Exec, "query" for the read family) and the
// statement, declared as the call's semantic identity — and hands the context
// down. It emits nothing itself: emission happens where the whole call is
// seen, which is the governance layer under it. [NewObsQuery] builds it.
type ObsQuery struct {
	stmt string
	next InnerQuery
}

// NewObsQuery builds the identity layer over next, declaring stmt on every
// terminal call.
func NewObsQuery(next InnerQuery, stmt string) *ObsQuery {
	return &ObsQuery{stmt: stmt, next: next}
}

// Release hands releaseRaw to the layer under it — this layer holds no
// resource.
func (o *ObsQuery) Release(releaseRaw bool) error { return o.next.Release(releaseRaw) }

func (o *ObsQuery) Exec(ctx context.Context) error {
	return o.next.Exec(observability.WithOperation(ctx, operation("exec", o.stmt)))
}

func (o *ObsQuery) Iter(ctx context.Context) *gocql.Iter {
	return o.next.Iter(observability.WithOperation(ctx, operation("query", o.stmt)))
}

func (o *ObsQuery) Scan(ctx context.Context, dest ...any) error {
	return o.next.Scan(observability.WithOperation(ctx, operation("query", o.stmt)), dest...)
}

func (o *ObsQuery) ScanCAS(ctx context.Context, dest ...any) (bool, error) {
	return o.next.ScanCAS(observability.WithOperation(ctx, operation("query", o.stmt)), dest...)
}

func (o *ObsQuery) MapScan(ctx context.Context, m map[string]any) error {
	return o.next.MapScan(observability.WithOperation(ctx, operation("query", o.stmt)), m)
}

func (o *ObsQuery) MapScanCAS(ctx context.Context, m map[string]any) (bool, error) {
	return o.next.MapScanCAS(observability.WithOperation(ctx, operation("query", o.stmt)), m)
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
