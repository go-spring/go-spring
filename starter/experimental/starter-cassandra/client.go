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
// wrapper Cassandra sessions are injected as, plus its lifecycle (Init/
// Destroy), the resource label, and the guard seam that routes statements
// through the resilience executor + observer. gocql exposes no reject-capable
// middleware, so the guard rides the guarded *Query wrapper every
// Client.Query/Client.Bind call returns (query.go) — coverage of the normal
// statement path is transparent, no opt-in helper required.
package StarterCassandra

import (
	"context"

	"github.com/gocql/gocql"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
)

// Client is the wrapper bean Cassandra sessions are injected as. It embeds
// the concrete *gocql.Session (so Query/Iter/Close and friends promote
// unchanged) and field-injects the observability policy. newClient returns
// one; gs field-injects Observability, then calls Init (InitMethod) to build
// the observer + executor.
type Client struct {
	*gocql.Session
	// Observability is field-injected by gs and configures the Exec helper's
	// observation (spans + metrics + access log).
	Observability observe.ObserveConfig `value:"${observability:=}"`

	// cfg is the connection config, retained for the resource label.
	cfg Config
	// exec is the resilience executor protecting Exec, resolved via
	// resilience.ExecutorFor; no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("cassandra:<hosts>") exec
	// scopes limiter/breaker state by.
	resource string
	// obs is the observer behind Exec.
	obs *observe.Observer
}

// Init is the gs InitMethod: gs field-injects Observability after newClient
// returns, then calls this. It builds the observer and resolves the executor
// through the neutral [resilience.ExecutorFor] seam (backed by
// starter-govern's governance center when imported), wraps it with the
// process-wide fault injector and observe-resilience. When governance is off
// the resolved executor is a transparent no-op.
func (o *Client) Init() error {
	o.obs = observe.NewDB("cassandra", o.Observability)
	o.resource = resilience.ResourceLabel("cassandra", o.cfg.Hosts[0])
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))
	exec = resilience.WrapExecutor(exec, "cassandra", o.Observability)
	o.exec = exec
	return nil
}

// Destroy is the gs destroy method: it closes the resilience executor (if
// armed) and the session.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	o.Session.Close()
	return nil
}

// guard routes call through the client's executor (when armed) and wraps it
// in an observation (when an observer is armed). It is the single guarded
// seam every statement path rides — the Query/Bind wrappers in query.go and
// the Exec alias below.
func (o *Client) guard(ctx context.Context, op, stmt string, call func(context.Context) error) error {
	if o.obs != nil {
		inner := call
		call = func(ctx context.Context) error {
			ctx, sp := o.obs.Start(ctx, op, stmt)
			err := inner(ctx)
			sp.End(err)
			return err
		}
	}
	if o.exec == nil {
		return call(ctx)
	}
	return o.exec.Execute(ctx, o.resource, call)
}

// Exec executes a statement synchronously through the guarded path. It is now
// a thin alias over the guarded Query wrapper (kept for callers written
// against the earlier opt-in helper); Query(...).Exec() is the same call.
// For iterators and paging use Query(...).Iter(), which is guarded too.
func (o *Client) Exec(ctx context.Context, stmt string, values ...any) error {
	return o.Query(stmt, values...).WithContext(ctx).Exec()
}
