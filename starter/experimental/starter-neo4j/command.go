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
// Query / RunWithResilience / StartSpan helpers that either declare a Neo4j
// operation's semantic identity (see observe.go) or route it through the
// resilience guard, plus the queryResilience guard that resolves the executor
// for a driver.
package StarterNeo4j

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
)

// Why a kit-backed Query entry (not a transparent driver wrapper):
//
// neo4j-go-driver's ExecuteQuery is a package-level generic free function, not a
// method on DriverWithContext, so a driver wrapper cannot intercept the main
// query path. Rather than hand-roll a fragile wrapper around every session/
// transaction method (only to still miss ExecuteQuery), the starter exposes a
// Query helper that declares the operation's identity and runs it under the
// resilience executor. Applications that want trace+metric+log call
// StarterNeo4j.Query instead of neo4j.ExecuteQuery directly — one symbol swap,
// full instrumentation. The declaration-only StartSpan helper remains for code
// that drives sessions manually and wants the same identity.

// Query runs a Cypher query via neo4j.ExecuteQuery, declaring the operation's
// semantic identity and routing it through the call-site resilience guard (rate
// limit / breaker / retry / bulkhead / timeout) when resilience is enabled for
// driver. It is the instrumented drop-in for neo4j.ExecuteQuery: same signature,
// plus automatic observability and protection. Because neo4j-go-driver's
// ExecuteQuery is a package-level function with no interception point, this is
// the resilience seam — applications swap neo4j.ExecuteQuery for
// StarterNeo4j.Query and pick up both. Code that drives sessions directly
// bypasses both unless it calls [RunWithResilience] / [StartSpan].
//
// The span, duration metrics and access log are not emitted here: declaring the
// identity is this layer's whole job now, and the resilience layer emits from
// the one point on the chain that sees the whole call, retries included.
func Query[T any](
	ctx context.Context,
	driver neo4j.DriverWithContext,
	query string,
	parameters map[string]any,
	newResultTransformer func() neo4j.ResultTransformer[T],
	settings ...neo4j.ExecuteQueryConfigurationOption,
) (T, error) {
	ctx = observability.WithOperation(ctx, operation("query", query))
	return resilience.Run(ctx, queryResilience(driver),
		func(ctx context.Context) (T, error) {
			return neo4j.ExecuteQuery[T](ctx, driver, query, parameters, newResultTransformer, settings...)
		})
}

// RunWithResilience runs fn through the resilience guard registered for driver,
// for code that drives sessions/transactions manually. It is the lower-level
// escape hatch for Neo4j operations that do not go through [Query] (e.g.
// driver.NewSession + session.Run / transactional callbacks). A raw driver (one
// that is not this starter's [Client]) still routes through an executor — the
// observed-only, loudly-unmanaged one (see [resilience.Unmanaged]) — rather than
// silently running bare, so the call is at least traced and the missing
// protection is announced. fn receives a context derived from ctx (the executor
// may derive a per-attempt timeout from it).
//
// Pair it with [StartSpan] to have the call declare its operation identity, so
// the resilience layer emits the neo4j signals (span, db.client.* metrics, the
// neo4j access log) for the manual call as well; without a declaration the
// layer falls back to its generic resilience.* signals.
func RunWithResilience(ctx context.Context, driver neo4j.DriverWithContext, fn func(context.Context) error) error {
	if exec := queryResilience(driver); exec != nil {
		return exec.Execute(ctx, fn)
	}
	return fn(ctx)
}

// StartSpan declares the semantic identity of a manual Neo4j operation (e.g. a
// session.Run / transaction callback the app drives directly) and returns the
// context carrying it. It starts nothing itself: run the operation under
// [RunWithResilience] and the resilience layer emits the span, the db.client.*
// metrics and the access log from the declaration — the starter only states what
// the operation is. op names the operation; summary is the Cypher text (recorded
// as db.statement, bounded).
func StartSpan(ctx context.Context, op, summary string) context.Context {
	return observability.WithOperation(ctx, operation(op, summary))
}

// queryResilience returns the executor for driver. On a *Client wrapper it is
// the executor fixed at construction (nil only for a hand-built zero Client, so
// callers guard against that). On any other driver — a raw neo4j.DriverWithContext
// passed directly — it returns [resilience.Unmanaged]: the driver has no config
// to derive a service label from, so it runs under the bare system label,
// observed and warned about once, instead of silently running with no telemetry
// at all.
func queryResilience(driver neo4j.DriverWithContext) resilience.ClientExecutor {
	if w, ok := driver.(*Client); ok {
		return w.exec
	}
	return resilience.Unmanaged(neo4jSystem, neo4jSystem)
}
