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
	"testing"

	"github.com/gomodule/redigo/redis"
)

// stubConn is a minimal redis.Conn recording the commands its Do receives.
type stubConn struct {
	redis.Conn
	calls []string // one entry per Do cmd
	reply interface{}
	err   error
}

func (s *stubConn) Do(cmd string, args ...interface{}) (interface{}, error) {
	s.calls = append(s.calls, cmd)
	return s.reply, s.err
}
func (s *stubConn) Close() error                      { return nil }
func (s *stubConn) Err() error                        { return nil }
func (s *stubConn) Send(string, ...interface{}) error { return nil }
func (s *stubConn) Flush() error                      { return nil }
func (s *stubConn) Receive() (interface{}, error)     { return nil, nil }

// newTestConn builds a Conn wired to inner with the given interceptor chain and
// no observe / no resilience — the minimal harness for command-path tests,
// routed through the real NewConn constructor.
func newTestConn(inner redis.Conn, chain ...CommandInterceptor) *Conn {
	return NewConn(inner, chain...)
}

// A short-circuiting interceptor must return its canned reply WITHOUT calling
// next, so the underlying conn is never reached.
func TestConn_InterceptorShortCircuitSkipsRedis(t *testing.T) {
	inner := &stubConn{reply: "from-redis"}
	calledNext := false
	chain := CommandInterceptor(func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
			if cmd == "GET" {
				return "cached", nil // local hit — do not call next
			}
			calledNext = true
			return next(ctx, cmd, args)
		}
	})
	c := newTestConn(inner, chain)

	reply, err := c.Do("GET", "k")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if reply != "cached" {
		t.Fatalf("reply = %v, want cached short-circuit", reply)
	}
	if len(inner.calls) != 0 {
		t.Fatalf("inner conn reached (%v); short-circuit must skip Redis", inner.calls)
	}
	if calledNext {
		t.Fatal("interceptor called next for GET; short-circuit must not")
	}
}

// An observer-style interceptor calls next; the command reaches the inner conn
// and its result flows back. A non-short-circuited cmd also reaches the conn.
func TestConn_InterceptorCallingNextReachesRedis(t *testing.T) {
	inner := &stubConn{reply: "from-redis"}
	chain := CommandInterceptor(func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
			reply, err := next(ctx, cmd, args)
			if err != nil {
				return nil, err
			}
			return "wrapped:" + reply.(string), nil
		}
	})
	c := newTestConn(inner, chain)

	reply, err := c.Do("SET", "k", "v")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if reply != "wrapped:from-redis" {
		t.Fatalf("reply = %v, want wrapped:from-redis", reply)
	}
	if len(inner.calls) != 1 || inner.calls[0] != "SET" {
		t.Fatalf("inner calls = %v, want exactly [SET]", inner.calls)
	}
}

// The interceptor chain composes onion-style: the FIRST registered interceptor is
// the OUTERMOST layer — it runs before inner ones on the way in and after them
// on the way out. An interceptor may also rewrite the command it forwards.
func TestConn_InterceptorOnionOrder(t *testing.T) {
	inner := &stubConn{reply: "r"}
	var order []string
	outer := CommandInterceptor(func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
			order = append(order, "outer-in")
			reply, err := next(ctx, cmd, args)
			order = append(order, "outer-out")
			return reply, err
		}
	})
	innerMw := CommandInterceptor(func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
			order = append(order, "inner-in")
			// rewrite: forward a different command toward Redis
			return next(ctx, "GET", args)
		}
	})
	c := newTestConn(inner, outer, innerMw)

	reply, err := c.Do("SET", "k", "v")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if reply != "r" {
		t.Fatalf("reply = %v, want r", reply)
	}
	want := []string{"outer-in", "inner-in", "outer-out"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
	// The inner interceptor rewrote the command the core executed.
	if len(inner.calls) != 1 || inner.calls[0] != "GET" {
		t.Fatalf("inner calls = %v, want exactly [GET] (rewritten)", inner.calls)
	}
}

// With no interceptors the chain is empty and the command reaches the conn.
func TestConn_NoInterceptorReachesRedis(t *testing.T) {
	inner := &stubConn{reply: 42}
	c := newTestConn(inner)

	reply, err := c.Do("INCR", "counter")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if reply != 42 {
		t.Fatalf("reply = %v, want 42", reply)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("inner calls = %v, want one", inner.calls)
	}
}

// With no resilience configured, the Conn wrapper is a
// pass-through: the command still reaches the inner conn and no span-related
// nil-deref occurs. Proves wrapping stays transparent.
func TestConn_ObserveDisabledPassThrough(t *testing.T) {
	inner := &stubConn{reply: "ok"}
	c := NewConn(inner)

	reply, err := c.Do("PING")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if reply != "ok" {
		t.Fatalf("reply = %v, want ok", reply)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("inner calls = %v, want one", inner.calls)
	}
}

// Pool.UseCommandInterceptor appends to the pool's chain and nil panics; a Conn
// dialed afterwards runs the chain, outermost ahead of the built-in layers.
func TestPool_UseCommandInterceptor(t *testing.T) {
	inner := &stubConn{reply: "ok"}
	p := &Pool{}
	p.UseCommandInterceptor(func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
			if cmd == "PING" {
				return "intercepted", nil
			}
			return next(ctx, cmd, args)
		}
	})
	if len(p.chain) != 1 {
		t.Fatalf("chain length = %d, want 1", len(p.chain))
	}

	c := NewConn(inner, p.chain...)
	reply, err := c.Do("PING")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if reply != "intercepted" {
		t.Fatalf("reply = %v, want intercepted", reply)
	}
	if len(inner.calls) != 0 {
		t.Fatalf("inner calls = %v, want none (short-circuit)", inner.calls)
	}

	var got any
	func() {
		defer func() { got = recover() }()
		p.UseCommandInterceptor(nil)
	}()
	if got == nil {
		t.Fatal("UseCommandInterceptor(nil) should panic")
	}
}

// wrapConn produces the instrumented Conn including the user interceptor chain.
func TestPool_wrapConn(t *testing.T) {
	inner := &stubConn{reply: "ok"}

	// With a chain armed, the factory's conn runs it (here: short-circuits).
	p := &Pool{}
	p.UseCommandInterceptor(func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
			return "intercepted", nil
		}
	})
	c := p.wrapConn(inner)
	if _, ok := c.(*Conn); !ok {
		t.Fatal("wrapConn must produce a *Conn")
	}
	reply, err := c.Do("PING")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if reply != "intercepted" {
		t.Fatalf("reply = %v, want intercepted (wrapConn must include the chain)", reply)
	}

	// Without a chain (and no observe/resilience) it is a plain *Conn wrapping
	// the raw connection.
	p2 := &Pool{}
	if _, ok := p2.wrapConn(inner).(*Conn); !ok {
		t.Fatal("default factory must yield a *Conn")
	}
}

// NewConn composes layers in order: an earlier layer sits outside a later
// one — the contract that makes the span-wrap-executor invariant (see
// Pool.wrapConn) expressible purely by layer order.
func TestNewConn_LayerOrder(t *testing.T) {
	inner := &stubConn{reply: "r"}
	var order []string
	mk := func(tag string) CommandInterceptor {
		return func(next CommandHandler) CommandHandler {
			return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
				order = append(order, tag+"-in")
				reply, err := next(ctx, cmd, args)
				order = append(order, tag+"-out")
				return reply, err
			}
		}
	}
	c := NewConn(inner, mk("first"), mk("second"))
	if _, err := c.Do("PING"); err != nil {
		t.Fatalf("Do: %v", err)
	}
	want := []string{"first-in", "second-in", "second-out", "first-out"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}
