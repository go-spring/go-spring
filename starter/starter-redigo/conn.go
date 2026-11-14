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
	"net"
	"time"

	"github.com/gomodule/redigo/redis"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/cloud/traffic"
)

// CommandHandler runs one Redis command. It is the pipeline's core signature:
// ctx, the peer the round addressed, cmd, and args are all in play, so each
// interceptor layer may rewrite any of them before handing the command toward
// Redis.
type CommandHandler func(
	ctx context.Context,
	peer chain.Peer,
	cmd string,
	args []interface{},
) (reply interface{}, err error)

// CommandInterceptor wraps a [CommandHandler] with additional behavior — the
// onion/chain model, the same shape as grpc's Interceptor. A layer may:
//   - run code before and after next (observe timing, mutate the reply, ...),
//   - rewrite the ctx / peer / cmd / args it passes to next,
//   - or return WITHOUT calling next to short-circuit (e.g. a local-cache hit
//     that must neither start a span nor consume a breaker permit).
//
// Conn's own capabilities are expressed as interceptors too: the command path
// is user interceptors (outermost) → resilience executor (declaration +
// emission) → the inner acquire-and-run call. Because user layers sit outside
// the executor, a short-circuit does NOT count toward the circuit breaker /
// rate limiter and does not emit a span; an observer-style layer that wants the
// command to count must call next.
//
// Compose via [Pool.UseCommandInterceptor]; the FIRST registered interceptor is
// the OUTERMOST layer.
type CommandInterceptor func(next CommandHandler) CommandHandler

// ErrPipelineUnsupported is what Send / Flush / Receive on a [Conn] return:
// the Conn holds no connection between commands — it acquires one per command,
// for the peer that round addressed — so queued pipelining has nothing to
// queue on. Issue pipelined commands through a raw redigo pool instead.
var ErrPipelineUnsupported = errors.New("redigo: pipelining is not supported on the governed Conn")

// Conn is the virtual connection [Pool.Get] hands out: it holds no connection
// of its own — every command runs the interceptor chain assembled from the
// pool (user interceptors, first-registered outermost; then the resilience
// layer with per-round addressing; then the acquire-and-run terminal) and
// acquires a pooled connection for the peer the round's addressing picked.
//
// It implements the optional DoContext / DoWithTimeout interfaces so the
// wrapper is transparent to type-asserting helpers (redis.DoContext etc.).
// Close is a no-op — nothing is held between commands — and Err is always nil.
//
// The executor sits INSIDE the span (span start → executor → inner call → span
// end), so one Execute covers any retries the policy drives, and each retry
// re-picks its peer. redis.ErrNil (a cache miss) is treated as success so it
// never trips the breaker — the redigo analog of gorm.ErrRecordNotFound;
// protection rejections (rate-limited / circuit-open / bulkhead-full) surface
// to the caller verbatim.
//
// Do and DoWithTimeout carry no context, so their spans are roots and the rule's
// timeout cannot interrupt them — prefer DoContext when either matters.
type Conn struct {
	// sink is the terminal acquire-and-run handler; wrap is the command chain
	// composed over it (user → resilience). nil wrap means a command IS the
	// sink call, with zero wrapper overhead.
	sink CommandHandler
	wrap CommandInterceptor
}

// NewConn composes the given interceptors (earlier = outermost) around sink,
// the terminal command handler. The layers are ordinary interceptors, any mix
// in any order. Pool.Get calls it for every virtual connection it hands out;
// it is equally usable standalone for hand assembly. With no layers the
// wrapper is a near-zero-cost pass-through.
func NewConn(sink CommandHandler, layers ...CommandInterceptor) *Conn {
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
	return &Conn{sink: sink, wrap: wrap}
}

// Do is the common, context-less path: its span is a root and the rule's timeout
// cannot interrupt it.
func (c *Conn) Do(cmd string, args ...interface{}) (interface{}, error) {
	return c.run(context.Background(), cmd, args, func(
		actx context.Context, peer chain.Peer, acmd string, aargs []interface{}) (interface{}, error) {
		return c.sink(actx, peer, acmd, aargs)
	})
}

// DoContext is the context-aware path; its span links to the caller and the
// rule's timeout can interrupt it.
func (c *Conn) DoContext(ctx context.Context, cmd string, args ...interface{}) (interface{}, error) {
	return c.run(ctx, cmd, args, func(
		actx context.Context, peer chain.Peer, acmd string, aargs []interface{}) (interface{}, error) {
		return c.sink(actx, peer, acmd, aargs)
	})
}

// DoWithTimeout instruments the timeout variant; its span is a root (same as Do)
// but the redigo read-timeout still bounds the call.
func (c *Conn) DoWithTimeout(timeout time.Duration, cmd string, args ...interface{}) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.DoContext(ctx, cmd, args...)
}

// run dispatches one command through the chain composed at construction. With
// every layer disabled (wrap nil) a command IS the sink — zero wrapper
// overhead — while the DoContext / DoWithTimeout interfaces stay transparent.
func (c *Conn) run(ctx context.Context, cmd string, args []interface{},
	call CommandHandler) (reply interface{}, err error) {

	h := call // terminal: the acquire-and-run invocation
	if c.wrap != nil {
		h = c.wrap(h)
	}
	return h(ctx, chain.Peer{}, cmd, args)
}

// Close returns the virtual connection. Nothing is held between commands —
// each command acquires and releases on its own — so this is a no-op.
func (c *Conn) Close() error { return nil }

// Err is always nil: the virtual connection holds no wire state that could
// fail between commands.
func (c *Conn) Err() error { return nil }

// Send / Flush / Receive are the pipelining half of redis.Conn, which the
// per-command acquire model cannot serve (see [ErrPipelineUnsupported]).
func (c *Conn) Send(string, ...interface{}) error { return ErrPipelineUnsupported }

// Flush is part of the unsupported pipelining half; see [ErrPipelineUnsupported].
func (c *Conn) Flush() error { return ErrPipelineUnsupported }

// Receive is part of the unsupported pipelining half; see [ErrPipelineUnsupported].
func (c *Conn) Receive() (interface{}, error) { return nil, ErrPipelineUnsupported }

// isTransientError reports whether the failure is a transport-level condition
// another attempt may survive — a dial/read/write error. Server replies
// (redis.Error: WRONGTYPE, ...) are definitive answers and are not retried.
func isTransientError(err error) bool {
	var ne net.Error
	return errors.As(err, &ne)
}

// resilienceInterceptor is the single command-chain layer: it builds the
// command's identity (see opFor in observe.go) and runs next under the executor
// (circuit breaker / rate limiter / bulkhead / retry, with addressing per
// round), passing the op explicitly so the executor both governs and emits
// from it. The executor derives its per-attempt context from ctx and hands
// each round the peer it addressed; the inner call receives that attempt
// context and peer, and routes with peer.Addr.
//
// A skipped command (skipOps) declares its identity like any other; only its
// success line is levelled to Debug.
//
// redis.ErrNil (a cache miss) is mapped to success so it never trips the
// breaker — the redigo analog of gorm.ErrRecordNotFound; protection rejections
// (rate-limited / circuit-open / bulkhead-full) surface to the caller verbatim.
// The executor plumbing (nil-as-success + rejection/fault translation) lives in
// [resilience.Run], shared with the other client adapters.
func resilienceInterceptor(exec chain.Executor, service string, prop traffic.Propagator) CommandInterceptor {
	return func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, _ chain.Peer, cmd string, args []interface{}) (interface{}, error) {
			// Name the service on the operation: the fault layer matches its rules
			// on it, and it sits outside this executor so it cannot borrow the
			// label from here.
			op := opFor(ctx, prop, cmd, args)
			op.Service = service
			return resilience.Run(ctx, exec, op,
				func(attemptCtx context.Context, peer chain.Peer) (interface{}, error) {
					return next(attemptCtx, peer, cmd, args)
				}, resilience.Tolerate(redis.ErrNil), resilience.RetryOn(isTransientError))
		}
	}
}
