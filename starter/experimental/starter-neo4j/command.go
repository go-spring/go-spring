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

// command.go is the "command seam" concept of this starter: the call-site
// Query / RunWithResilience / Operation helpers that either build a Neo4j
// operation's semantic identity (see observe.go) or route a call through the
// resilience guard. Each takes the starter's [Client] — the only client this
// seam accepts — so a call routed through it is always the observed, governed
// one; a raw driver is not accepted, since a call outside the client is a call
// outside its governance.

package StarterNeo4j

import (
	"context"

	"go-spring.org/cloud/runtime/dataplane/chain"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud/runtime/dataplane/resilience"
)

// Why a kit-backed Query entry (not a transparent driver wrapper):
//
// neo4j-go-driver's ExecuteQuery is a package-level generic free function, not a
// method on DriverWithContext, so a driver wrapper cannot intercept the main
// query path. Rather than hand-roll a fragile wrapper around every session/
// transaction method (only to still miss ExecuteQuery), the starter exposes a
// Query helper that builds the operation's identity and runs it under the
// resilience executor. Applications that want trace+metric+log call
// StarterNeo4j.Query instead of neo4j.ExecuteQuery, passing the injected
// *StarterNeo4j.Client. [Operation] builds the same identity for code that
// drives sessions manually and runs it under [RunWithResilience].

// Query runs a Cypher query via neo4j.ExecuteQuery, building the operation's
// semantic identity and routing it through the call-site resilience guard (rate
// limit / breaker / retry / bulkhead / timeout) when resilience is enabled for
// client. It is the instrumented counterpart of neo4j.ExecuteQuery — the same
// query arguments, but taking the starter's [Client] so the call always runs
// under that client's executor. A raw driver is deliberately NOT accepted: a
// call outside the client is a call outside its governance and observation.
// Because neo4j-go-driver's ExecuteQuery is a package-level function with no
// interception point, this call-site helper is the resilience seam. Code that
// drives sessions directly bypasses both unless it calls
// [Operation] / [RunWithResilience].
//
// The operation's identity is built here and passed EXPLICITLY to the executor
// (not carried on the context); the span, duration metrics and access log are
// emitted by the resilience layer from the one point on the chain that sees the
// whole call, retries included.
func Query[T any](
	ctx context.Context,
	client *Client,
	query string,
	parameters map[string]any,
	newResultTransformer func() neo4j.ResultTransformer[T],
	settings ...neo4j.ExecuteQueryConfigurationOption,
) (T, error) {
	op := operation("query", query)
	op.Service = client.serviceLabel
	return resilience.Run(ctx, client.exec, op,
		func(ctx context.Context) (T, error) {
			return neo4j.ExecuteQuery(ctx, client, query, parameters, newResultTransformer, settings...)
		})
}

// RunWithResilience runs fn through client's resilience guard for the operation
// op, for code that drives sessions/transactions manually. It is the lower-level
// escape hatch for Neo4j operations that do not go through [Query] (e.g.
// client.NewSession + session.Run / transactional callbacks). fn receives a
// context derived from ctx (the executor may derive a per-attempt timeout from
// it). A [Client] with no executor attached (governance off, a hand-built zero
// client) runs fn inline unchanged.
//
// Build op with [Operation] to have the call declare its operation identity, so
// the resilience layer emits the neo4j signals (span, db.client.* metrics, the
// neo4j access log) for the manual call as well; pass the zero [chain.Operation]
// and the layer falls back to its generic resilience.* signals.
func RunWithResilience(ctx context.Context, client *Client, op chain.Operation, fn func(context.Context) error) error {
	if exec := client.exec; exec != nil {
		op.Service = client.serviceLabel
		return exec.Execute(ctx, op, nil, fn)
	}
	return fn(ctx)
}

// Operation builds the semantic identity of a manual Neo4j operation (e.g. a
// session.Run / transaction callback the app drives directly) for passing to
// [RunWithResilience]. It builds nothing else: run the operation under
// [RunWithResilience] and the resilience layer emits the span, the db.client.*
// metrics and the access log from the identity — the starter only states what
// the operation is. op names the operation; summary is the Cypher text (recorded
// as db.statement, bounded).
func Operation(op, summary string) chain.Operation {
	return operation(op, summary)
}
