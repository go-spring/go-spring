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
// Query / RunWithResilience / StartSpan / EndSpan helpers that route Neo4j
// operations through this starter's own instrumentation (see observe.go) and
// the resilience guard, plus the
// queryResilience guard that resolves the executor for a driver.
package StarterNeo4j

import (
	"context"
	"sync"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud/governance/resilience"
)

// Why a kit-backed Query entry (not a transparent driver wrapper):
//
// neo4j-go-driver's ExecuteQuery is a package-level generic free function, not a
// method on DriverWithContext, so a driver wrapper cannot intercept the main
// query path. Rather than hand-roll a fragile wrapper around every session/
// transaction method (only to still miss ExecuteQuery), the starter exposes a
// Query helper that wraps ExecuteQuery with the observe kit. Applications that
// want trace+metric+log call StarterNeo4j.Query instead of neo4j.ExecuteQuery
// directly — one symbol swap, full instrumentation. The low-level StartSpan/
// EndSpan helpers remain for code that drives sessions manually.
//
// A package-level default observer (see observe.go) backs both — there is no
// per-instance wiring because the instrumentation cannot bind to a free-function
// call path. It rides the OTel globals starter-otel installs.

// defaultObs builds the shared observer lazily on first use, so its OTel
// instruments bind to whatever meter provider is current then, not to the
// noop global of package init.
var defaultObs = sync.OnceValue(func() *dbObserver { return newDBObserver("neo4j") })

// Query runs a Cypher query via neo4j.ExecuteQuery, wrapped with the starter's
// instrumentation (trace span + duration/in-flight metric + access log) and, when resilience is
// enabled for driver, the call-site resilience guard (rate limit / breaker /
// retry / bulkhead / timeout). It is the instrumented drop-in for
// neo4j.ExecuteQuery: same signature, plus automatic observability and
// protection. Because neo4j-go-driver's ExecuteQuery is a package-level
// function with no interception point, this is the resilience seam —
// applications swap neo4j.ExecuteQuery for StarterNeo4j.Query and pick up both
// observability and resilience. Code that drives sessions directly bypasses
// both unless it calls [RunWithResilience].
func Query[T any](
	ctx context.Context,
	driver neo4j.DriverWithContext,
	query string,
	parameters map[string]any,
	newResultTransformer func() neo4j.ResultTransformer[T],
	settings ...neo4j.ExecuteQueryConfigurationOption,
) (T, error) {
	ctx, sp := defaultObs().Start(ctx, "query", query)
	var res T
	var err error
	if exec, resource := queryResilience(driver); exec != nil {
		err = exec.Execute(ctx, resource, func(ctx context.Context) error {
			res, err = neo4j.ExecuteQuery[T](ctx, driver, query, parameters, newResultTransformer, settings...)
			return err
		})
	} else {
		res, err = neo4j.ExecuteQuery[T](ctx, driver, query, parameters, newResultTransformer, settings...)
	}
	sp.End(err)
	return res, err
}

// RunWithResilience runs fn through the resilience guard registered for driver,
// for code that drives sessions/transactions manually. It is the lower-level
// escape hatch for Neo4j operations that do not go through [Query] (e.g.
// driver.NewSession + session.Run / transactional callbacks). When no executor
// is registered for driver, fn runs unprotected. fn receives a context derived
// from ctx (the executor may derive a per-attempt timeout from it).
func RunWithResilience(ctx context.Context, driver neo4j.DriverWithContext, fn func(context.Context) error) error {
	if exec, resource := queryResilience(driver); exec != nil {
		return exec.Execute(ctx, resource, fn)
	}
	return fn(ctx)
}

// StartSpan starts a client observation for a manual Neo4j operation (e.g. a
// session.Run / transaction callback the app drives directly). End the returned
// span once it completes. op names the operation; summary is the Cypher text
// (recorded as db.statement, bounded).
func StartSpan(ctx context.Context, op, summary string) (context.Context, *dbSpan) {
	return defaultObs().Start(ctx, op, summary)
}

// EndSpan records err (if any) on the span and ends it.
func EndSpan(span *dbSpan, err error) {
	span.End(err)
}

// queryResilience returns the executor + resource on a wrapped driver, or
// (nil, "") when the driver is not a *Client wrapper (e.g. a raw neo4j driver
// passed directly). On a wrapped driver the executor is always resolved (a
// no-op when governance is off), so callers route through it unconditionally.
func queryResilience(driver neo4j.DriverWithContext) (resilience.Executor, string) {
	if w, ok := driver.(*Client); ok {
		return w.exec, w.resource
	}
	return nil, ""
}
