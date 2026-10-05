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

// client.go is the "resource entity" concept of this starter: the Client
// wrapper Cassandra sessions are injected as, its lifecycle (construction/
// Close), and the statement builders that run through per-query chains
// (query.go). gocql exposes no reject-capable middleware, so the chain rides
// the guarded *Query wrapper every Client.Query/Client.Bind call returns —
// coverage of the normal statement path is transparent, no opt-in helper
// required.

package StarterCassandra

import (
	"context"
	"go-spring.org/cloud/chain"

	"github.com/gocql/gocql"
	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
)

// Client is the wrapper bean Cassandra sessions are injected as. It holds the
// raw *gocql.Session as the exported [Client.Session] — a read-only handle,
// never to run statements through, that would bypass the chains — plus what
// every per-query chain is assembled around. [NewClient] is the only way to
// build a Client, so a client can never exist without its identity.
//
// The type is exported because gocql (like gomemcache) offers no
// reject-capable hook/plugin point, so the only way to observe and protect
// per-statement traffic is to hold the wrapper itself. Apps therefore inject
// *Client rather than *gocql.Session. [Driver.CreateClient] returns this type
// too, so a custom driver works with the same type the ecosystem sees.
//
// gocql exposes more than the guarded statement path (session tuning, batches,
// metadata). None of that promotes — the session is a plain field — so each of
// those methods is delegated explicitly below.
type Client struct {
	// Session is the raw gocql session — the original object, not a wrapper.
	// A handle to READ (session tuning goes through the delegations below),
	// never to run statements through: that would bypass the chain.
	Session *gocql.Session

	// exec is the resilience executor protecting every statement, set by
	// [NewClient] from the governance bundle — an observed-only,
	// loudly-unmanaged executor when the bundle is zero. Held for teardown
	// only: the chains are per-query, so there is no session-level chain head
	// whose Release could close it — [Client.Close] is that release point.
	exec chain.Executor
}

// NewClient builds a complete Client — identity, governance and all — over a
// connected raw session. session must be ready for use — it is normally the
// Driver's product. The wrapper derives its service label from the connection
// config.
//
// params carries the container's facilities (see [cloud.ClientParams]), and is
// applied HERE so a Client cannot exist half-assembled: there is no Init step,
// no later patching, and nothing the container has to remember to call. A
// hand-built client passes the zero [cloud.ClientParams]; its executor then degrades to
// resilience.Unmanaged — observed, with a one-time warning that no protection
// applies — rather than silently running bare.
//
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the call order relative to the center's wiring is
// irrelevant.
func NewClient(session *gocql.Session, cfg Config, params cloud.ClientParams) *Client {
	// The service label prefers the first contact point. Guard the index so a
	// malformed Config (empty hosts, which the value:"${hosts}" expr is meant to
	// reject) degrades to the bare prefix instead of panicking in the ctor.
	var host string
	if len(cfg.Hosts) > 0 {
		host = cfg.Hosts[0]
	}
	serviceLabel := resilience.ServiceLabel("cassandra", host)
	return &Client{
		Session: session,
		exec:    params.ExecutorFor("cassandra", serviceLabel),
	}
}

// Close releases the resilience executor and closes the session. It is the gs
// destroy method, and the one place the client touches the executor: the
// chains are per-query, so the executor's session-level release point lives
// here rather than in a layer. gocql's Close is idempotent, so calling Close
// twice is safe apart from the executor already being closed (best-effort).
func (o *Client) Close() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	o.Session.Close()
	return nil
}

// Query mirrors (*gocql.Session).Query but returns a guarded wrapper instead
// of a raw *gocql.Query, so the normal statement path is transparently
// protected. It is the only signature change versus a raw session — see the
// Query type comment in query.go for the chaining caveat.
func (o *Client) Query(stmt string, values ...any) *Query {
	q := &Query{Query: o.Session.Query(stmt, values...), stmt: stmt}
	q.InnerQuery = o.chain(q)
	return q
}

// Bind mirrors (*gocql.Session).Bind for the same reason as Query: bound
// statements get the same transparent chain.
func (o *Client) Bind(stmt string, b func(q *gocql.QueryInfo) ([]any, error)) *Query {
	q := &Query{Query: o.Session.Bind(stmt, b), stmt: stmt}
	q.InnerQuery = o.chain(q)
	return q
}

// chain assembles one query's chain: identity over governance over the query's
// own raw adapter.
func (o *Client) chain(q *Query) InnerQuery {
	return NewObsQuery(NewGuardQuery(NewRawQuery(q), o.exec), q.stmt)
}

// Exec executes a statement synchronously through the guarded path. It is a
// thin alias over the guarded Query wrapper (kept for callers written against
// the earlier opt-in helper); Query(...).Exec() is the same call. For iterators
// and paging use Query(...).Iter(), which is guarded too.
func (o *Client) Exec(ctx context.Context, stmt string, values ...any) error {
	return o.Query(stmt, values...).WithContext(ctx).Exec()
}

// --- Explicit delegation of the raw session's remaining methods ---
//
// The raw session is no longer embedded, so nothing is promoted: a caller that
// used a *gocql.Session method through the wrapper would otherwise lose it when
// the wrapper became a Client. Each method below is a verbatim pass-through,
// preserving the pre-wrapper behavior (these are session-level knobs, batches
// and metadata — they never rode the guard, and still do not).

// AwaitSchemaAgreement waits for schema agreement across the cluster.
func (o *Client) AwaitSchemaAgreement(ctx context.Context) error {
	return o.Session.AwaitSchemaAgreement(ctx)
}

// SetConsistency sets the default consistency level for the session.
func (o *Client) SetConsistency(cons gocql.Consistency) { o.Session.SetConsistency(cons) }

// SetPageSize sets the default page size for queries on the session.
func (o *Client) SetPageSize(n int) { o.Session.SetPageSize(n) }

// SetPrefetch sets the default prefetch (fraction of the page size) for the
// session.
func (o *Client) SetPrefetch(p float64) { o.Session.SetPrefetch(p) }

// SetTrace sets the tracer the session reports query traces to.
func (o *Client) SetTrace(trace gocql.Tracer) { o.Session.SetTrace(trace) }

// Closed reports whether the session has been closed.
func (o *Client) Closed() bool { return o.Session.Closed() }

// KeyspaceMetadata returns the metadata for the named keyspace.
func (o *Client) KeyspaceMetadata(keyspace string) (*gocql.KeyspaceMetadata, error) {
	return o.Session.KeyspaceMetadata(keyspace)
}

// NewBatch creates a new batch of the given type.
func (o *Client) NewBatch(typ gocql.BatchType) *gocql.Batch { return o.Session.NewBatch(typ) }

// ExecuteBatch executes a batch, atomically by default. Batch execution does
// not ride the guarded statement path (the same coverage the pre-wrapper client
// had): route individual statements through [Client.Query]/[Client.Exec] when
// per-statement resilience is wanted.
func (o *Client) ExecuteBatch(batch *gocql.Batch) error { return o.Session.ExecuteBatch(batch) }

// ExecuteBatchCAS executes a batch as a light-weight transaction, returning
// whether it was applied and the result iterator.
func (o *Client) ExecuteBatchCAS(batch *gocql.Batch, dest ...any) (applied bool, iter *gocql.Iter, err error) {
	return o.Session.ExecuteBatchCAS(batch, dest...)
}

// MapExecuteBatchCAS executes a batch as a light-weight transaction, scanning
// the first row into dest.
func (o *Client) MapExecuteBatchCAS(batch *gocql.Batch, dest map[string]any) (applied bool, iter *gocql.Iter, err error) {
	return o.Session.MapExecuteBatchCAS(batch, dest)
}
