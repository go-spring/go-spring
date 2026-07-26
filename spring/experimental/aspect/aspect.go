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

// Package aspect provides the Go-idiomatic equivalent of Spring AOP: a way to
// attach cross-cutting concerns (transaction, cache, audit, timing, rate limit)
// to methods and handlers without tangling them into business code.
//
// It deliberately does not replicate Java AOP. There is no bytecode weaving and
// no runtime dynamic proxy. The mechanism is an explicit, type-safe interceptor
// chain plus a decorator convention wired through the existing dependency
// injection container. The result is the same effect — a concern declared once,
// applied around many operations, invisible to the caller — reached with plain
// Go composition instead of reflection.
//
// # The chain
//
// A [Chain] is an ordered list of [Interceptor]s wrapped around a target
// function. Index 0 is the outermost wrapper. Each interceptor receives a
// [Joinpoint] and decides whether (and with what context) to call
// [Joinpoint.Proceed], and what to do with its result. A nil or empty chain is a
// transparent pass-through: the target runs directly, so wiring stays a no-op
// until an interceptor is configured.
//
//	chain := aspect.NewChain(
//	    aspect.Recover(),
//	    aspect.Transactional(txManager),
//	)
//	order, err := aspect.Around(chain, ctx, "PlaceOrder",
//	    func(ctx context.Context) (*Order, error) {
//	        return repo.Insert(ctx, o)
//	    })
//
// The chain boundary carries the result as an any so an interceptor such as
// [Cache] can read or replace it; [Around] restores static typing at the call
// site with a single type assertion and no reflection.
//
// # Decorator + DI convention
//
// To make a concern invisible to callers, keep the business type behind an
// interface and register a thin decorator that wraps the concrete
// implementation, sharing the same interface:
//
//	type OrderService interface {
//	    Place(ctx context.Context, o *Order) error
//	}
//
//	type orderService struct{ /* deps */ }
//	func (s *orderService) Place(ctx context.Context, o *Order) error { ... }
//
//	type orderServiceAspect struct {
//	    inner OrderService
//	    chain *aspect.Chain
//	}
//	func (a *orderServiceAspect) Place(ctx context.Context, o *Order) error {
//	    return a.chain.RunE(ctx, "OrderService.Place",
//	        func(ctx context.Context) error { return a.inner.Place(ctx, o) })
//	}
//
// Wire it with the container so consumers inject OrderService and never see the
// decorator:
//
//	gs.Provide(&orderService{}).Name("orderServiceImpl")
//	gs.Provide(func(inner *orderService, chain *aspect.Chain) OrderService {
//	    return &orderServiceAspect{inner: inner, chain: chain}
//	}).Export(gs.As[OrderService]())
//
// For HTTP handlers use [NewHandler] instead of a decorator; it turns each
// request into a joinpoint that flows through the chain.
package aspect

import (
	"context"
	"slices"
)

// Joinpoint describes the single operation currently being intercepted. It is
// created by [Chain.Run] and threaded through every [Interceptor] in the chain.
type Joinpoint struct {
	// Context is the context for this invocation. An interceptor may replace it
	// before calling Proceed to propagate values downstream (for example a
	// transaction handle) — see [Transactional].
	Context context.Context

	// Method is a logical name for the intercepted operation. It is used purely
	// for interceptor decisions (cache keys, pointcut matching, audit labels);
	// the chain never interprets it.
	Method string

	// Result holds the target's return value once Proceed has run. An interceptor
	// may read it after Proceed (for example to cache it) or set it and skip
	// Proceed entirely to short-circuit the invocation (for example a cache hit).
	Result any

	proceed func(context.Context) (any, error)
}

// Proceed invokes the next interceptor in the chain, or the target function when
// this is the innermost interceptor. It runs the remaining chain with ctx, so an
// interceptor can pass down a derived context. The return value is also stored in
// [Joinpoint.Result] so later logic in the same interceptor can inspect it.
func (jp *Joinpoint) Proceed(ctx context.Context) (any, error) {
	jp.Context = ctx
	v, err := jp.proceed(ctx)
	jp.Result = v
	return v, err
}

// Interceptor runs cross-cutting logic around a [Joinpoint]. Implementations
// call [Joinpoint.Proceed] to continue the chain, or return a result directly to
// short-circuit it. Implementations must be safe for concurrent use.
type Interceptor interface {
	Intercept(jp *Joinpoint) (any, error)
}

// InterceptorFunc adapts an ordinary function to the [Interceptor] interface.
type InterceptorFunc func(jp *Joinpoint) (any, error)

// Intercept calls f(jp).
func (f InterceptorFunc) Intercept(jp *Joinpoint) (any, error) { return f(jp) }

// Chain is an immutable, ordered set of interceptors applied around a target.
// The interceptor at index 0 is the outermost wrapper and runs first. The zero
// value (and a nil *Chain) is a valid transparent pass-through.
type Chain struct {
	interceptors []Interceptor
}

// NewChain builds a chain from the given interceptors, in outermost-first order.
// Nil interceptors are ignored so optional stages can be spread in without guards.
func NewChain(interceptors ...Interceptor) *Chain {
	c := &Chain{}
	for _, i := range interceptors {
		if i != nil {
			c.interceptors = append(c.interceptors, i)
		}
	}
	return c
}

// With returns a new chain with the given interceptors appended after the
// existing ones (inner side). The receiver is left unchanged, so a base chain can
// be shared and specialized per call site.
func (c *Chain) With(interceptors ...Interceptor) *Chain {
	nc := NewChain(interceptors...)
	if c == nil {
		return nc
	}
	merged := make([]Interceptor, 0, len(c.interceptors)+len(nc.interceptors))
	merged = append(merged, c.interceptors...)
	merged = append(merged, nc.interceptors...)
	return &Chain{interceptors: merged}
}

// Run executes target under the chain, scoping interceptor decisions to method.
// It composes the interceptors so index 0 is outermost. A nil/empty chain calls
// target directly, making the wiring a no-op until interceptors are configured.
func (c *Chain) Run(ctx context.Context, method string, target func(context.Context) (any, error)) (any, error) {
	if c == nil || len(c.interceptors) == 0 {
		return target(ctx)
	}
	jp := &Joinpoint{Context: ctx, Method: method}
	// Build the proceed closure from the innermost interceptor outward so that
	// calling jp.Proceed enters interceptor 0 first.
	next := target
	for _, interceptor := range slices.Backward(c.interceptors) {
		inner := next
		next = func(ctx context.Context) (any, error) {
			jp.proceed = inner
			jp.Context = ctx
			return interceptor.Intercept(jp)
		}
	}
	return next(ctx)
}

// RunE is the error-only convenience form of [Chain.Run] for operations that
// return no meaningful value (commands, void handlers).
func (c *Chain) RunE(ctx context.Context, method string, target func(context.Context) error) error {
	_, err := c.Run(ctx, method, func(ctx context.Context) (any, error) {
		return nil, target(ctx)
	})
	return err
}

// Around runs a strongly typed target function through the chain. It boxes the
// T result across the any-typed chain boundary and asserts it back on the way
// out, so call sites stay type-safe with no reflection. If an interceptor
// short-circuits with a value that is not a T (or with nil), the zero value of T
// is returned alongside any error.
func Around[T any](c *Chain, ctx context.Context, method string, fn func(context.Context) (T, error)) (T, error) {
	v, err := c.Run(ctx, method, func(ctx context.Context) (any, error) {
		return fn(ctx)
	})
	result, _ := v.(T)
	return result, err
}
