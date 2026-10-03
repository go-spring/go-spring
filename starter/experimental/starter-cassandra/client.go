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
// wrapper Cassandra sessions are injected as, plus its lifecycle (construction/
// Destroy), the service label, and the guard seam that declares each statement's
// identity and routes it through the resilience executor. gocql exposes no
// reject-capable middleware, so the guard rides the guarded *Query wrapper every
// Client.Query/Client.Bind call returns (query.go) — coverage of the normal
// statement path is transparent, no opt-in helper required.
package StarterCassandra

import (
	"context"
	"go-spring.org/cloud/chain"

	"github.com/gocql/gocql"
	"go-spring.org/cloud"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
)

// Client is the wrapper bean Cassandra sessions are injected as. The raw
// *gocql.Session is an unexported field, not an embedded one: [NewClient] is
// the only way to build a Client, so a client can never exist without its
// identity (the service label), and the method surface below is the whole API.
// There is deliberately no exported accessor for the raw session: that would let
// a caller bypass the declaration and governance layers without it showing up in
// review.
//
// The type is exported because gocql (like gomemcache) offers no
// reject-capable hook/plugin point, so the only way to observe and protect
// per-statement traffic is to hold the wrapper itself. Apps therefore inject
// *Client rather than *gocql.Session. [Driver.CreateClient] returns this type
// too, so a custom driver works with the same type the ecosystem sees.
//
// gocql exposes more than the guarded statement path (session tuning, batches,
// metadata). None of that promotes any more — the field is unexported — so
// each of those methods is delegated explicitly below.
type Client struct {
	// session is the raw gocql session. Unexported so [NewClient] is the only
	// constructor — see the type doc.
	session *gocql.Session

	// serviceLabel is the resilience service key ("cassandra:<host>") exec
	// scopes limiter/breaker state by. Fixed by [NewClient] from the connection
	// config.
	serviceLabel string

	// exec is the resilience executor protecting every statement, set by
	// [NewClient] from the governance bundle — an observed-only, loudly-unmanaged
	// executor when the bundle is zero. It is also the single emitter of the
	// statement's span, metrics and access log.
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
		session:      session,
		serviceLabel: serviceLabel,
		exec:         params.ExecutorFor("cassandra", serviceLabel),
	}
}

// Destroy releases the resilience executor and closes the session. It is the gs
// destroy method.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	o.session.Close()
	return nil
}

// guard declares the statement's identity and routes call through the client's
// executor. It is the single guarded seam every statement path rides — the
// Query/Bind wrappers in query.go and the Exec alias below.
//
// The span, duration metrics and access log are not emitted here: declaring the
// identity is this layer's whole job now, and the resilience layer (inside exec)
// emits from the one point on the chain that sees the whole call, retries
// included. When the client runs unmanaged (a zero governance bundle) the
// executor is an observed-only pass-through, so the call runs with a single
// function-call overhead.
func (o *Client) guard(ctx context.Context, op, stmt string, call func(context.Context) error) error {
	ctx = observability.WithOperation(ctx, operation(op, stmt))
	return o.exec.Execute(ctx, call)
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
	return o.session.AwaitSchemaAgreement(ctx)
}

// SetConsistency sets the default consistency level for the session.
func (o *Client) SetConsistency(cons gocql.Consistency) { o.session.SetConsistency(cons) }

// SetPageSize sets the default page size for queries on the session.
func (o *Client) SetPageSize(n int) { o.session.SetPageSize(n) }

// SetPrefetch sets the default prefetch (fraction of the page size) for the
// session.
func (o *Client) SetPrefetch(p float64) { o.session.SetPrefetch(p) }

// SetTrace sets the tracer the session reports query traces to.
func (o *Client) SetTrace(trace gocql.Tracer) { o.session.SetTrace(trace) }

// Close closes the session's connections. It exists because the raw session has
// it: the field is unexported, so nothing is promoted and the method would
// otherwise be reachable only through [Client.Destroy]. gocql documents that
// Close is idempotent, so calling it here and again from Destroy is safe.
func (o *Client) Close() { o.session.Close() }

// Closed reports whether the session has been closed.
func (o *Client) Closed() bool { return o.session.Closed() }

// KeyspaceMetadata returns the metadata for the named keyspace.
func (o *Client) KeyspaceMetadata(keyspace string) (*gocql.KeyspaceMetadata, error) {
	return o.session.KeyspaceMetadata(keyspace)
}

// NewBatch creates a new batch of the given type.
func (o *Client) NewBatch(typ gocql.BatchType) *gocql.Batch { return o.session.NewBatch(typ) }

// ExecuteBatch executes a batch, atomically by default. Batch execution does
// not ride the guarded statement path (the same coverage the pre-wrapper client
// had): route individual statements through [Client.Query]/[Client.Exec] when
// per-statement resilience is wanted.
func (o *Client) ExecuteBatch(batch *gocql.Batch) error { return o.session.ExecuteBatch(batch) }

// ExecuteBatchCAS executes a batch as a light-weight transaction, returning
// whether it was applied and the result iterator.
func (o *Client) ExecuteBatchCAS(batch *gocql.Batch, dest ...any) (applied bool, iter *gocql.Iter, err error) {
	return o.session.ExecuteBatchCAS(batch, dest...)
}

// MapExecuteBatchCAS executes a batch as a light-weight transaction, scanning
// the first row into dest.
func (o *Client) MapExecuteBatchCAS(batch *gocql.Batch, dest map[string]any) (applied bool, iter *gocql.Iter, err error) {
	return o.session.MapExecuteBatchCAS(batch, dest)
}
