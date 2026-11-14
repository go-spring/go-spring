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
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
)

// netConn is the pool's connection type in tests: an id.
type netConn int64

// newTestPool builds a pool whose dial function counts dials and closes and
// refuses the address "down".
func newTestPool(t *testing.T, tune ...func(*PoolConfig[netConn])) (*Pool[netConn], *atomic.Int64, *atomic.Int64) {
	t.Helper()
	var dialed, closed atomic.Int64
	pcfg := PoolConfig[netConn]{
		Dial: func(_ context.Context, addr string) (netConn, error) {
			if addr == "down" {
				dialed.Add(1)
				return 0, errors.New("refused")
			}
			return netConn(dialed.Add(1)), nil
		},
		CloseConn: func(netConn) error {
			closed.Add(1)
			return nil
		},
	}
	for _, f := range tune {
		f(&pcfg)
	}
	p, err := NewPool(pcfg)
	assert.That(t, err).Nil()
	return p, &dialed, &closed
}

func TestPoolAcquireDialAndIdleHit(t *testing.T) {
	p, dialed, _ := newTestPool(t)

	r1, err := p.Acquire(context.Background(), "a")
	assert.That(t, err).Nil()
	assert.Number(t, int64(r1.conn)).Equal(1)

	r1.Release()
	r2, err := p.Acquire(context.Background(), "a")
	assert.That(t, err).Nil()
	// The released connection is parked and reused: no second dial.
	assert.Number(t, dialed.Load()).Equal(1)
	assert.Number(t, r2.conn).Equal(netConn(1))
	r2.Release()
}

func TestPoolMaxActiveExhaustion(t *testing.T) {
	// Exhausting the endpoint's budget is what must refuse the acquire.
	p, _, _ := newTestPool(t, func(c *PoolConfig[netConn]) { c.MaxActive = 1 })

	r1, err := p.Acquire(context.Background(), "a")
	assert.That(t, err).Nil()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = p.Acquire(ctx, "a")
	assert.Error(t, err).Is(ErrPoolExhausted)
	// The refusal is a LOCAL limit: no evidence against the endpoint, and the
	// sentinel still matches through the mark.
	assert.That(t, chain.IsResourceError(err)).True()

	r1.Release()
	r2, err := p.Acquire(context.Background(), "a")
	assert.That(t, err).Nil()
	r2.Release()
}

func TestPoolCloseDiscards(t *testing.T) {
	p, dialed, closed := newTestPool(t)
	r1, err := p.Acquire(context.Background(), "a")
	assert.That(t, err).Nil()

	// A discarded connection is closed, not parked: the next acquire dials.
	r1.Close()
	assert.Number(t, closed.Load()).Equal(1)
	r2, err := p.Acquire(context.Background(), "a")
	assert.That(t, err).Nil()
	assert.Number(t, dialed.Load()).Equal(2)
	r2.Release()

	// Closing the pool closes parked idles and refuses later acquires.
	assert.That(t, p.Close()).Nil()
	assert.That(t, p.Close()).Nil() // idempotent
	_, err = p.Acquire(context.Background(), "a")
	assert.Error(t, err).Is(ErrPoolClosed)
}

func TestPoolDialFailureIsConnectMarked(t *testing.T) {
	// A dial failure surfaces as a ConnectError-marked error — the chain's
	// settlement reads it as connect evidence — and the pool does not walk to
	// another endpoint: addressing is the chain's, not the pool's.
	p, _, _ := newTestPool(t)

	_, err := p.Acquire(context.Background(), "down")
	assert.That(t, err != nil).True()
	assert.That(t, chain.IsConnectError(err)).True()
}

// TestPoolCallerAbortDuringDialIsNoEvidence pins a dial cut off by the
// CALLER's context: it is not connect evidence (the peer was never given a
// chance to refuse) and not a local limit either.
func TestPoolCallerAbortDuringDialIsNoEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	p, err := NewPool(PoolConfig[netConn]{
		Dial: func(dctx context.Context, _ string) (netConn, error) {
			close(started)
			<-dctx.Done()
			return 0, dctx.Err()
		},
		CloseConn: func(netConn) error { return nil },
	})
	assert.That(t, err).Nil()

	go func() {
		<-started
		cancel()
	}()
	_, err = p.Acquire(ctx, "a")
	assert.That(t, err != nil).True()
	assert.That(t, chain.IsConnectError(err)).False()
	assert.That(t, chain.IsResourceError(err)).False()
	assert.That(t, errors.Is(err, context.Canceled)).True()
}

// TestPoolDialBudgetIsConnectEvidence pins the other side: a dial budget the
// POOL added expiring is a connect failure, never a caller abort.
func TestPoolDialBudgetIsConnectEvidence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, err := NewPool(PoolConfig[netConn]{
			Dial: func(ctx context.Context, _ string) (netConn, error) {
				<-ctx.Done()
				return 0, ctx.Err()
			},
			CloseConn:   func(netConn) error { return nil },
			DialTimeout: time.Second,
		})
		assert.That(t, err).Nil()

		_, err = p.Acquire(context.Background(), "a")
		assert.That(t, err != nil).True()
		assert.That(t, chain.IsConnectError(err)).True()
	})
}

func TestPoolIdleExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var closed atomic.Int64
		p, err := NewPool(PoolConfig[netConn]{
			Dial:        func(context.Context, string) (netConn, error) { return 1, nil },
			CloseConn:   func(netConn) error { closed.Add(1); return nil },
			IdleTimeout: time.Second,
		})
		assert.That(t, err).Nil()

		r, err := p.Acquire(context.Background(), "a")
		assert.That(t, err).Nil()
		r.Release()
		assert.Number(t, closed.Load()).Equal(0)

		time.Sleep(2 * time.Second)
		r2, err := p.Acquire(context.Background(), "a")
		assert.That(t, err).Nil()
		// The parked connection expired: the acquire closed it and dialed a
		// fresh one.
		assert.Number(t, closed.Load()).Equal(1)
		r2.Release()
	})
}

func TestPoolPerEndpointIsolation(t *testing.T) {
	// Each endpoint keeps its own MaxActive budget: holding one connection to
	// "a" does not consume "b"'s slot.
	p, err := NewPool(PoolConfig[netConn]{
		Dial:      func(_ context.Context, addr string) (netConn, error) { return 1, nil },
		CloseConn: func(netConn) error { return nil },
		MaxActive: 1,
	})
	assert.That(t, err).Nil()

	for _, addr := range []string{"a", "b"} {
		r, err := p.Acquire(context.Background(), addr)
		assert.That(t, err).Nil()
		defer r.Release()
	}

	// Both endpoints' budgets are spent: another acquire is refused as a
	// local exhaustion, never as an endpoint failure.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = p.Acquire(ctx, "a")
	assert.Error(t, err).Is(ErrPoolExhausted)
}

func TestDialAddrContextRoundTrip(t *testing.T) {
	ctx := WithDialAddr(context.Background(), "10.0.0.1:6379")
	addr, ok := DialAddrFrom(ctx)
	assert.That(t, ok).True()
	assert.That(t, addr).Equal("10.0.0.1:6379")

	_, ok = DialAddrFrom(context.Background())
	assert.That(t, ok).False()
}

// TestPoolAcquireHonorsConnectBudget pins the budget's effect at the pool: a
// staged governance budget bounds the whole establishment, tighter than the
// pool's own timeouts — an endpoint that never answers is refused at the
// budget, not at the dial timeout.
func TestPoolAcquireHonorsConnectBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, err := NewPool(PoolConfig[netConn]{
			Dial: func(ctx context.Context, _ string) (netConn, error) {
				<-ctx.Done() // blackhole: never answers, dies only by deadline
				return 0, ctx.Err()
			},
			CloseConn:   func(netConn) error { return nil },
			DialTimeout: 5 * time.Second, // the static cap the budget must beat
		})
		assert.That(t, err).Nil()

		start := time.Now()
		rd := &chain.Round{ConnectTimeout: 200 * time.Millisecond}
		_, err = p.Acquire(chain.WithRound(context.Background(), rd), "a")
		assert.That(t, err != nil).True()
		assert.That(t, time.Since(start) < time.Second).True() // the budget fired, not the 5s cap
	})
}
