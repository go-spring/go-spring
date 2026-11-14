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

package connpool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
)

// ErrPoolExhausted is the pool's verdict that no connection could be acquired
// in bounds: the endpoint's MaxActive slots are in use and the acquire
// outlived its budget (the caller's context, or AcquireTimeout when set). It
// is a LOCAL condition — the endpoint is not to blame — so the chain settles
// it as no evidence against the peer, never as a downstream failure.
var ErrPoolExhausted = errors.New("connpool: pool exhausted")

// ErrPoolClosed is the pool's refusal to acquire after [Pool.Close]. Closing
// is a lifecycle fact, not a health signal: it is never settled.
var ErrPoolClosed = errors.New("connpool: pool closed")

// PoolConfig configures a [Pool]. C is the connection type the pool hands
// out; the dial and close functions are the only protocol the pool knows.
type PoolConfig[C any] struct {
	// Dial opens one connection to addr. It runs under the pool's dial
	// timeout; the connection it returns must be usable by CloseConn.
	// Required.
	Dial func(ctx context.Context, addr string) (C, error)

	// CloseConn releases one connection. Required — the pool cannot know how
	// a protocol's connection closes.
	CloseConn func(C) error

	// MaxActive is the concurrent-connection ceiling PER ENDPOINT. 0 adopts
	// the default (8).
	MaxActive int

	// MaxIdle is the idle-connection ceiling per endpoint. 0 adopts the
	// default (2).
	MaxIdle int

	// IdleTimeout is how long an idle connection survives before the next
	// Acquire or Release sweep closes it. 0 adopts the default (5m). Reaping
	// is lazy — no background goroutine — so an idle pool costs nothing.
	IdleTimeout time.Duration

	// DialTimeout bounds one dial. 0 adopts the default (3s).
	DialTimeout time.Duration

	// AcquireTimeout bounds one Acquire overall (waiting for a slot plus
	// dialing). 0 means only the caller's context bounds it.
	AcquireTimeout time.Duration

	// Observer receives the pool's acquire/idle events. Nil uses the package's
	// default OTel observer; an application may supply its own adapter.
	Observer Observer
}

// Pool-level defaults.
const (
	defaultMaxActive   = 8
	defaultMaxIdle     = 2
	defaultIdleTimeout = 5 * time.Minute
	defaultDialTimeout = 3 * time.Second
)

// idleConn is one parked connection with the moment it went idle.
type idleConn[C any] struct {
	conn    C
	idledAt time.Time
}

// endpointPool is one endpoint's slice of the pool: the MaxActive semaphore
// and the idle stack (LIFO, so the most recently parked connection is the
// first reused).
type endpointPool[C any] struct {
	sem  chan struct{}
	idle []idleConn[C]
}

// Pool is the connection-holding layer for a client with no vendor pool of
// its own: it keeps idle connections per endpoint and bounds concurrency per
// endpoint. Addressing is NOT its business — the chain's addressing stage
// picks the endpoint and hands its address to [Pool.Acquire], so a retry
// re-picks and a failing peer is excluded by the picker, not walked to here.
//
// The pool owns connections and nothing else. Health stays in the balancer's
// tracker, protection stays wherever the composition root put it — this
// layer's only vocabulary is acquire, dial, park, close. A dial failure is
// marked [chain.ConnectError] so the chain's round settlement reads it as the
// strong evidence it is; the pool never settles anything itself.
type Pool[C any] struct {
	dial      func(ctx context.Context, addr string) (C, error)
	closeConn func(C) error

	maxActive      int
	maxIdle        int
	idleTimeout    time.Duration
	dialTimeout    time.Duration
	acquireTimeout time.Duration

	obs Observer

	mu      sync.Mutex
	subs    map[string]*endpointPool[C]
	idleCnt int
	closed  bool
}

// NewPool builds a [Pool] from cfg, applying the defaults. The dial and close
// functions are required.
func NewPool[C any](cfg PoolConfig[C]) (*Pool[C], error) {
	if cfg.Dial == nil || cfg.CloseConn == nil {
		return nil, errutil.Explain(nil, "connpool: pool requires non-nil dial and close functions")
	}
	normalize := func(v, def int) int {
		if v <= 0 {
			return def
		}
		return v
	}
	p := &Pool[C]{
		dial:           cfg.Dial,
		closeConn:      cfg.CloseConn,
		maxActive:      normalize(cfg.MaxActive, defaultMaxActive),
		maxIdle:        normalize(cfg.MaxIdle, defaultMaxIdle),
		idleTimeout:    cfg.IdleTimeout,
		dialTimeout:    cfg.DialTimeout,
		acquireTimeout: cfg.AcquireTimeout,
		subs:           map[string]*endpointPool[C]{},
	}
	if p.idleTimeout <= 0 {
		p.idleTimeout = defaultIdleTimeout
	}
	if p.dialTimeout <= 0 {
		p.dialTimeout = defaultDialTimeout
	}
	p.obs = cfg.Observer
	return p, nil
}

// observeAcquire reports one acquire attempt; a nil observer disables it.
func (p *Pool[C]) observeAcquire(ctx context.Context, outcome string) {
	if p.obs != nil {
		p.obs.Acquire(ctx, outcome)
	}
}

// observeIdle moves the idle gauge; a nil observer disables it.
func (p *Pool[C]) observeIdle(delta int64) {
	if p.obs != nil {
		p.obs.Idle(context.Background(), delta)
	}
}

// dialAddrKey is the context key a command path uses to tell its vendor
// driver's dial hook which address the chain's addressing stage picked.
type dialAddrKey struct{}

// WithDialAddr returns a context carrying addr for the dial hook to read (see
// [DialAddrFrom]). It is the seam between a chain that picked a peer and a
// driver that dials deep inside its own call stack — the vendor pool between
// them would otherwise choose an address of its own.
func WithDialAddr(ctx context.Context, addr string) context.Context {
	return context.WithValue(ctx, dialAddrKey{}, addr)
}

// DialAddrFrom reads the address [WithDialAddr] staged, reporting whether one
// was staged at all. A dial hook that finds none dials on its own — the call
// did not run under a chain that addresses (a warm-up dial, a plain client).
func DialAddrFrom(ctx context.Context) (string, bool) {
	addr, ok := ctx.Value(dialAddrKey{}).(string)
	return addr, ok
}

// ConnRef is one acquired connection. The holder returns it with
// [ConnRef.Release] (park it for reuse) or discards it with [ConnRef.Close].
// Both are idempotent; the first call wins.
type ConnRef[C any] struct {
	pool *Pool[C]
	ep   *endpointPool[C]
	addr string
	conn C
	done bool
}

// Acquire returns one usable connection to addr — the endpoint the chain's
// addressing stage picked for this round: an idle one when the endpoint has
// one parked, a freshly dialed one otherwise, waiting for an endpoint slot
// bounded by the caller's context (or AcquireTimeout).
//
// The returned error is one of: [ErrPoolExhausted] (local condition, never
// the endpoint's fault), [ErrPoolClosed], or the dial error wrapped in
// [chain.ConnectError] so the chain's settlement counts it as connect evidence.
// When the CALLER's own context ends first, the error is that context's error
// instead: walking away is neither a local limit nor evidence about the peer.
func (p *Pool[C]) Acquire(ctx context.Context, addr string) (*ConnRef[C], error) {
	// The caller's context, before the budgets this pool and the round add
	// below: only ITS expiry means the caller walked away, and the error kind
	// depends on that distinction.
	parent := ctx
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		// Marked like exhaustion: a closed pool is OUR lifecycle fact, not the
		// endpoint's failure, so a round that races the teardown settles no
		// evidence against the peer.
		return nil, chain.ResourceError(ErrPoolClosed)
	}
	if p.acquireTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.acquireTimeout)
		defer cancel()
	}
	// The governance connection budget, when the round staged one, bounds the
	// whole establishment — the slot wait and the dial beneath it. WithTimeout
	// only tightens: an already-closer deadline (the round's own) keeps winning.
	if rd, ok := chain.RoundFrom(ctx); ok && rd.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rd.ConnectTimeout)
		defer cancel()
	}
	return p.acquireFrom(ctx, parent, addr)
}

// acquireFrom acquires from one endpoint: slot, idle hit or dial. parent is the
// caller's context before any budget was layered on; only its expiry means the
// caller walked away rather than the pool's own budget firing.
func (p *Pool[C]) acquireFrom(ctx, parent context.Context, addr string) (*ConnRef[C], error) {
	ep := p.subPoolFor(addr)
	if ep == nil {
		return nil, chain.ResourceError(ErrPoolClosed)
	}
	select {
	case ep.sem <- struct{}{}:
	case <-ctx.Done():
		p.observeAcquire(ctx, "exhausted")
		// Marked as a LOCAL limit: our ceiling refused, the endpoint was never
		// asked anything, so the chain's settlement counts no evidence against
		// the peer. errors.Is(err, ErrPoolExhausted) still holds through
		// Unwrap.
		return nil, chain.ResourceError(ErrPoolExhausted)
	}
	// Idle hit: the sweep inside park/put keeps the stack fresh, so the top
	// entry is either reusable or dropped here.
	if conn, ok := p.popIdle(addr, ep); ok {
		p.observeAcquire(ctx, "hit")
		return &ConnRef[C]{pool: p, ep: ep, addr: addr, conn: conn}, nil
	}
	// Dial under the pool's dial timeout, on the round's context. The failure
	// is MARKED, not settled: the chain settles the round once, and the mark
	// is what makes the dial legible to that settlement.
	dctx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline || deadlineAfter(ctx, p.dialTimeout) {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(ctx, p.dialTimeout)
		defer cancel()
	}
	conn, derr := p.dial(dctx, addr)
	if derr != nil {
		<-ep.sem
		p.observeAcquire(ctx, "dial_failed")
		// A dial the CALLER's context cut off is not evidence against the
		// peer: the refusal never got a chance to happen. A budget this pool
		// or the round added still reads as a connect failure.
		if parent.Err() != nil {
			return nil, fmt.Errorf("%w: %w", parent.Err(), derr)
		}
		return nil, chain.ConnectError(derr)
	}
	p.observeAcquire(ctx, "dialed")
	return &ConnRef[C]{pool: p, ep: ep, addr: addr, conn: conn}, nil
}

// deadlineAfter reports whether ctx's deadline, if any, sits beyond d.
func deadlineAfter(ctx context.Context, d time.Duration) bool {
	dl, ok := ctx.Deadline()
	return ok && time.Until(dl) > d
}

// subPoolFor returns addr's endpoint pool, creating it on first use. It
// returns nil for a closed pool — Acquire has already refused that case, so
// the nil only reaches a caller that raced the close.
func (p *Pool[C]) subPoolFor(addr string) *endpointPool[C] {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	ep, ok := p.subs[addr]
	if !ok {
		ep = &endpointPool[C]{sem: make(chan struct{}, p.maxActive)}
		p.subs[addr] = ep
	}
	return ep
}

// popIdle takes the newest reusable idle connection for addr, closing any
// expired ones it passes.
func (p *Pool[C]) popIdle(addr string, ep *endpointPool[C]) (C, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(ep.idle) > 0 {
		top := ep.idle[len(ep.idle)-1]
		ep.idle = ep.idle[:len(ep.idle)-1]
		p.idleCnt--
		p.observeIdle(-1)
		if time.Since(top.idledAt) > p.idleTimeout {
			_ = p.closeConn(top.conn)
			continue
		}
		return top.conn, true
	}
	var zero C
	return zero, false
}

// park returns conn to addr's idle stack, closing it when the stack is full.
// It reports whether the connection was parked.
func (p *Pool[C]) park(addr string, ep *endpointPool[C], conn C) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || len(ep.idle) >= p.maxIdle {
		return false
	}
	ep.idle = append(ep.idle, idleConn[C]{conn: conn, idledAt: time.Now()})
	p.idleCnt++
	p.observeIdle(1)
	return true
}

// Raw returns the acquired connection itself. The holder runs its protocol on
// it and then returns the ref with [ConnRef.Release] or [ConnRef.Close].
func (r *ConnRef[C]) Raw() C { return r.conn }

// Release returns the connection to the pool: parked for reuse when the
// endpoint's idle stack has room, closed otherwise. Idempotent.
func (r *ConnRef[C]) Release() {
	if r == nil || r.done {
		return
	}
	r.done = true
	if r.pool.park(r.addr, r.ep, r.conn) {
		<-r.ep.sem
		return
	}
	r.Close()
}

// Close discards the connection outright: the holder knows the connection is
// bad (a protocol error, a broken stream), so parking it would hand the next
// acquirer a dead connection. Idempotent.
func (r *ConnRef[C]) Close() {
	if r == nil || r.done {
		return
	}
	r.done = true
	_ = r.pool.closeConn(r.conn)
	<-r.ep.sem
}

// PoolStats is the pool's live accounting, for readiness pages and examples.
type PoolStats struct {
	Active    int // connections currently held by acquirers
	Idle      int // connections parked for reuse
	Endpoints int // endpoints with a sub-pool
}

// Stats reports the pool's live accounting.
func (p *Pool[C]) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := PoolStats{Idle: p.idleCnt, Endpoints: len(p.subs)}
	for _, ep := range p.subs {
		s.Active += len(ep.sem)
	}
	return s
}

// Close closes the pool: every idle connection is closed and later acquires
// fail fast. Closing is idempotent.
func (p *Pool[C]) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	for addr, ep := range p.subs {
		for _, ic := range ep.idle {
			_ = p.closeConn(ic.conn)
		}
		ep.idle = nil
		delete(p.subs, addr)
	}
	count := p.idleCnt
	p.idleCnt = 0
	p.mu.Unlock()
	if count > 0 {
		p.observeIdle(int64(-count))
	}
	return nil
}
