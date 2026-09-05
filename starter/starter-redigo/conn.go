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

package StarterRedigo

import (
	"context"
	"errors"
	"time"

	"github.com/gomodule/redigo/redis"
	"go-spring.org/cloud/governance/resilience"
)

// CommandHandler runs one Redis command. It is the pipeline's core signature:
// ctx, cmd, and args are all in play, so each interceptor layer may rewrite any
// of them before handing the command toward Redis.
type CommandHandler func(
	ctx context.Context,
	cmd string,
	args []interface{},
) (reply interface{}, err error)

// CommandInterceptor wraps a [CommandHandler] with additional behavior — the
// onion/chain model, the same shape as grpc's Interceptor. A layer may:
//   - run code before and after next (observe timing, mutate the reply, ...),
//   - rewrite the ctx / cmd / args it passes to next,
//   - or return WITHOUT calling next to short-circuit (e.g. a local-cache hit
//     that must neither start a span nor consume a breaker permit).
//
// Conn's own capabilities are expressed as interceptors too: the command path
// is user interceptors (outermost) → observe span → resilience executor → the
// inner call. Because user layers sit outside the executor, a short-circuit
// does NOT count toward the circuit breaker / rate limiter and does NOT emit a
// span; an observer-style layer that wants the command to count must call next.
//
// Compose via [Pool.UseCommandInterceptor]; the FIRST registered interceptor is
// the OUTERMOST layer.
type CommandInterceptor func(next CommandHandler) CommandHandler

// Conn wraps a redis.Conn so every command flows through an interceptor chain
// assembled once at construction (see Pool.wrapConn): user interceptors (via
// [Pool.UseCommandInterceptor], first-registered outermost), then the observe
// layer (trace span + duration metric + access log, see observe.go), then the
// resilience layer (executor), then the inner call — all uniform
// [CommandInterceptor] layers. Conn itself holds only the composed wrap.
//
// It implements the optional DoContext / DoWithTimeout interfaces so the wrapper
// is transparent to type-asserting helpers (redis.DoContext etc.): when the
// inner conn supports them the call is instrumented with the caller's context
// (linking the client span to the request trace); otherwise it falls back to the
// plain Do path.
//
// The executor sits INSIDE the span (span start → executor → inner call → span
// end), so one Execute covers any retries the policy drives. redis.ErrNil (a
// cache miss) is treated as success so it never trips the breaker — the redigo
// analog of gorm.ErrRecordNotFound; protection rejections (rate-limited /
// circuit-open / bulkhead-full) surface to the caller verbatim.
//
// Do and DoWithTimeout carry no context, so their spans are roots and an
// attempt-timeout cannot interrupt them — prefer DoContext when either matters.
// Send / Flush / Receive (pipelining) are left to the embedded Conn
// uninstrumented; correlating queued Sends with their Receives needs deeper,
// out-of-scope instrumentation.
type Conn struct {
	redis.Conn
	// wrap is the command chain composed once at construction
	// (user → observe → resilience). nil when every layer is disabled — then a
	// command IS the inner call, with zero wrapper overhead.
	wrap CommandInterceptor
}

// NewConn wraps a raw redis.Conn, composing the given interceptors (earlier =
// outermost) around the inner call. The layers are ordinary interceptors, any
// mix in any order. Pool.newConn calls it for every connection the pool hands
// out; it is equally usable standalone for hand assembly. With no layers the
// wrapper is a near-zero-cost pass-through.
func NewConn(raw redis.Conn, layers ...CommandInterceptor) *Conn {
	var wrap CommandInterceptor
	if len(layers) > 0 {
		ls := layers
		wrap = func(next CommandHandler) CommandHandler {
			// fold from the innermost: the first layer ends up outermost.
			for i := len(ls) - 1; i >= 0; i-- {
				next = ls[i](next)
			}
			return next
		}
	}
	return &Conn{Conn: raw, wrap: wrap}
}

// Do is the common, context-less path: its span is a root and an attempt-timeout
// cannot interrupt it. It is mandatory (part of the redis.Conn interface) and the
// fallback for the optional DoContext / DoWithTimeout when the inner conn lacks them.
func (c *Conn) Do(cmd string, args ...interface{}) (interface{}, error) {
	return c.run(context.Background(), cmd, args, func(
		actx context.Context, acmd string, aargs []interface{}) (interface{}, error) {
		return c.Conn.Do(acmd, aargs...)
	})
}

// DoContext instruments the context-aware path; its span links to the caller and
// an attempt-timeout can interrupt it. Falls back to Do when the inner conn has
// no DoContext.
func (c *Conn) DoContext(ctx context.Context, cmd string, args ...interface{}) (interface{}, error) {
	type ctxDoer interface {
		DoContext(context.Context, string, ...interface{}) (interface{}, error)
	}
	inner, ok := c.Conn.(ctxDoer)
	if !ok {
		return c.Do(cmd, args...)
	}
	return c.run(ctx, cmd, args, func(
		actx context.Context, acmd string, aargs []interface{}) (interface{}, error) {
		return inner.DoContext(actx, acmd, aargs...)
	})
}

// DoWithTimeout instruments the timeout variant; its span is a root (same as Do)
// but the redigo read-timeout still bounds the call. Falls back to Do when the
// inner conn has no DoWithTimeout.
func (c *Conn) DoWithTimeout(timeout time.Duration, cmd string, args ...interface{}) (interface{}, error) {
	type timeoutDoer interface {
		DoWithTimeout(time.Duration, string, ...interface{}) (interface{}, error)
	}
	inner, ok := c.Conn.(timeoutDoer)
	if !ok {
		return c.Do(cmd, args...)
	}
	return c.run(context.Background(), cmd, args, func(
		actx context.Context, acmd string, aargs []interface{}) (interface{}, error) {
		return inner.DoWithTimeout(timeout, acmd, aargs...)
	})
}

// run dispatches one command through the chain composed at construction. With
// every layer disabled (wrap nil) a command IS the inner call — zero wrapper
// overhead — while the DoContext / DoWithTimeout interfaces stay transparent.
func (c *Conn) run(ctx context.Context, cmd string, args []interface{},
	call CommandHandler) (reply interface{}, err error) {

	h := call // terminal: the inner command invocation
	if c.wrap != nil {
		h = c.wrap(h)
	}
	return h(ctx, cmd, args)
}

// resilienceInterceptor is the resilience layer of the command chain: it runs
// next under the executor (circuit breaker / rate limiter / bulkhead / retry).
// The executor derives its per-attempt context from ctx; the inner call receives
// that attempt context, which only DoContext threads through (the others ignore
// it — redigo's Do and DoWithTimeout take no context).
//
// redis.ErrNil (a cache miss) is mapped to success so it never trips the
// breaker — the redigo analog of gorm.ErrRecordNotFound; protection rejections
// (rate-limited / circuit-open / bulkhead-full) surface to the caller verbatim.
// The executor plumbing (nil-as-success + rejection/fault translation) lives in
// [resilience.Run], shared with the other client adapters.
func resilienceInterceptor(exec resilience.Executor, resource string) CommandInterceptor {
	return func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
			return resilience.Run(ctx, exec, resource, func(e error) bool {
				return errors.Is(e, redis.ErrNil)
			}, func(attemptCtx context.Context) (interface{}, error) {
				return next(attemptCtx, cmd, args)
			})
		}
	}
}
