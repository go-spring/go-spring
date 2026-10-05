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

// deadline.go carries the request BUDGET across a hop boundary: how much time
// the caller has left, so the callee can bound its own work by that instead of
// by a fresh allowance of its own.
//
// Inside one process this needs no code — a derived context's deadline is the
// earlier of its own and its parent's, so a call nested under a bounded one is
// already bounded, retries included. Across processes it needs a wire format,
// which is what this file adds: the caller writes what is left into the
// request's metadata, and the callee reads it back into its own context.
//
// Without it a chain of N hops each configured with the same timeout costs up to
// N times that timeout, and every hop keeps working on a request its caller
// abandoned long ago. With it the chain spends one budget, and the hop that
// runs out is the one that stops.

package resilience

import (
	"context"
	"strconv"
	"time"

	"go-spring.org/cloud/propagate"
)

// BudgetHeader is the metadata key the remaining budget travels under, in whole
// milliseconds.
//
// It is transport metadata, not a protocol: gRPC already carries a deadline in
// its own header and needs none of this, while HTTP has nothing equivalent —
// hence one name, defined here, that both ends of an HTTP hop agree on.
//
// The value is a downgradeable hint, never an authority: a hop may always bound
// itself more tightly (its own policy does), and a hop that cannot read it is
// simply unbounded by its caller, which is the behaviour that existed before.
const BudgetHeader = "go-spring-budget-ms"

// InjectBudget writes ctx's remaining budget into c, so the next hop budgets
// inside what is left rather than starting an allowance of its own.
//
// A context with no deadline writes nothing: "no budget" and "no time left" are
// different statements, and a zero would read as the second. A budget that has
// already run out is written as 1ms rather than 0, so the value stays a
// well-formed positive number; it expires on arrival either way, which is the
// truth of the situation.
func InjectBudget(ctx context.Context, c propagate.Carrier) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return
	}
	ms := time.Until(deadline).Milliseconds()
	if ms < 1 {
		ms = 1
	}
	c.Set(BudgetHeader, strconv.FormatInt(ms, 10))
}

// WithBudget returns ctx bounded by the budget c carries, together with the
// cancel that releases it. Callers must call cancel when the operation ends.
//
// The returned context is the earlier of the caller's budget and whatever ctx
// already carried — Go's own rule for derived contexts, and the reason a hop can
// never extend the allowance its caller left it, only shorten it.
//
// A carrier without a budget, or with a value that is malformed, zero or
// negative, returns ctx unchanged with a cancel that does nothing: the value
// comes from outside the process, and the only safe reading of a claim this
// layer cannot use is to ignore it. A larger budget than ctx already has is
// likewise ignored — a caller cannot be given more time than it took.
func WithBudget(ctx context.Context, c propagate.Carrier) (context.Context, context.CancelFunc) {
	vs := c.Values(BudgetHeader)
	if len(vs) == 0 {
		return ctx, func() {}
	}
	ms, err := strconv.ParseInt(vs[0], 10, 64)
	if err != nil || ms <= 0 {
		return ctx, func() {}
	}
	deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)
	if d, ok := ctx.Deadline(); ok && !deadline.Before(d) {
		return ctx, func() {}
	}
	return context.WithDeadline(ctx, deadline)
}
