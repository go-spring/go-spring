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

package loadbalance

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	balancer "go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
)

// withLoop mimics the governance retry loop around one round: it CREATES the round
// before the executor runs — the round addressing stages its pick on — and SEALS
// the measured facts when it returns. Addressing records nothing itself.
func withLoop(rec *chain.Recorder, run func() error) error {
	rd := rec.NewRound()
	start := time.Now()
	err := run()
	rd.Start = start
	rd.Duration = time.Since(start)
	rd.Status = chain.ClassifyStatus(err)
	rd.ErrorType = chain.RoundErrorType(err)
	rd.Error = err
	return err
}

// runRound runs one round of the executor and returns the address it staged.
func runRound(t *testing.T, exec chain.Executor, ctx context.Context) string {
	t.Helper()
	rec := new(chain.Recorder)
	var addr string
	err := withLoop(rec, func() error {
		return exec.Execute(ctx, chain.Operation{}, rec, func(_ context.Context, rd *chain.Round) error {
			addr = rd.Peer.Addr
			return nil
		})
	})
	assert.Error(t, err).Nil()
	return addr
}

// TestRoundPicksAndSettlesOutcome pins the executor's two duties in one round:
// the pick is staged on the round before fn runs, and the round's outcome is
// settled with the peer's owner — a success settles nil, a failure reaches the
// tracker. The chain is the only addressing face, so these tests drive a real
// one rather than a double.
func TestRoundPicksAndSettlesOutcome(t *testing.T) {
	ctx := context.Background()
	m := newManager(t)
	err := m.Apply(ctx, Settings{Selection: Selection{Balancer: balancer.RoundRobin, RequestFailures: 1, SuspendDuration: time.Hour, MaxEjectionPercent: 100}})
	assert.That(t, err).Nil()
	c := mustChain(t, m, ctx, staticSource(eps("a")...), "demo:service")

	var gotAddr string
	var called bool
	rec := new(chain.Recorder)
	err = withLoop(rec, func() error {
		return Wrap(c, nil).Execute(ctx, chain.Operation{}, rec, func(_ context.Context, rd *chain.Round) error {
			called = true
			gotAddr = rd.Peer.Addr
			return nil
		})
	})
	assert.Error(t, err).Nil()
	assert.That(t, called).True()
	assert.String(t, gotAddr).Equal("a")
	// A successful round settles nil: the tracker learned no failure.
	assert.That(t, suspended(c.cur.Load().tracker, "a")).False()

	// A failing round settles the error: the same endpoint is now suspended.
	rec = new(chain.Recorder)
	_ = withLoop(rec, func() error {
		return Wrap(c, nil).Execute(ctx, chain.Operation{}, rec, func(context.Context, *chain.Round) error {
			return errors.New("boom")
		})
	})
	assert.That(t, suspended(c.cur.Load().tracker, "a")).True()
}

// TestFailedPeerIsExcludedFromNextRound pins the failed-list semantics: the
// round carries the staged pick's address, the next pick of THE SAME call
// receives it as the exclusion list, and lands elsewhere.
func TestFailedPeerIsExcludedFromNextRound(t *testing.T) {
	ctx := context.Background()
	m := newManager(t)
	c := mustChain(t, m, ctx, staticSource(eps("a", "b")...), "demo:service")
	exec := Wrap(c, nil)

	boom := errors.New("boom")
	rec := &chain.Recorder{}
	var first string
	err := withLoop(rec, func() error {
		return exec.Execute(ctx, chain.Operation{}, rec, func(_ context.Context, rd *chain.Round) error {
			first = rd.Peer.Addr
			return boom
		})
	})
	assert.Error(t, err).Is(boom)
	assert.That(t, first == "a" || first == "b").True()
	// The round carries the staged pick's address, and the next call's pick is
	// handed it as the exclusion list.
	assert.Slice(t, rec.FailedAddrs()).Equal([]string{first})

	// The next round of THE SAME call (a retry shares the recorder) is handed
	// the failed peer as its exclusion list and lands on the other endpoint.
	var second string
	err = withLoop(rec, func() error {
		return exec.Execute(ctx, chain.Operation{}, rec, func(_ context.Context, rd *chain.Round) error {
			second = rd.Peer.Addr
			return nil
		})
	})
	assert.Error(t, err).Nil()
	assert.That(t, second != first).True()
}

// TestNoEndpointsRefusesTheRound proves a pick that finds nothing fails the
// round before fn runs, with the chain's own sentinel.
func TestNoEndpointsRefusesTheRound(t *testing.T) {
	ctx := context.Background()
	m := newManager(t)
	c := mustChain(t, m, ctx, staticSource(), "demo:service")

	rec := new(chain.Recorder)
	called := false
	err := withLoop(rec, func() error {
		return Wrap(c, nil).Execute(ctx, chain.Operation{}, rec, func(context.Context, *chain.Round) error {
			called = true
			return nil
		})
	})
	assert.That(t, called).False()
	assert.Error(t, err).Is(chain.ErrNoEndpoints)
	last := rec.LastRound()
	assert.That(t, last != nil).True()
	assert.That(t, last.Peer.Addr).Equal("")
	assert.String(t, string(last.ErrorType)).Equal("no_endpoints")
}

// TestLoopRecordInheritsStagedPeerAddr pins that addressing stages the peer on
// the round the loop created, so the sealed record reports where the round ran.
func TestLoopRecordInheritsStagedPeerAddr(t *testing.T) {
	ctx := context.Background()
	m := newManager(t)
	c := mustChain(t, m, ctx, staticSource(eps("a")...), "demo:service")

	rec := new(chain.Recorder)
	_ = withLoop(rec, func() error {
		return Wrap(c, nil).Execute(ctx, chain.Operation{}, rec, func(context.Context, *chain.Round) error {
			return errors.New("boom")
		})
	})
	assert.That(t, rec.Rounds[0].Peer.Addr).Equal("a") // staged by addressing, sealed by the loop
}

// TestRoundRoutesByAffinityKey pins the routing input the executor reads off the
// caller's options: a stated HashKey routes through the keyed strategy (same key
// → same endpoint, different keys may differ), and a caller stating none picks
// unkeyed (the strategy's own fallback).
func TestRoundRoutesByAffinityKey(t *testing.T) {
	ctx := context.Background()
	m := newManager(t)
	err := m.Apply(ctx, Settings{Selection: Selection{Balancer: balancer.ConsistentHash}})
	assert.That(t, err).Nil()
	c := mustChain(t, m, ctx, staticSource(eps("a", "b", "c")...), "demo:service")
	exec := Wrap(c, nil)

	withKey := func(k string) context.Context {
		return chain.WithOptions(context.Background(), chain.Options{HashKey: k})
	}

	// Same key maps to the same endpoint, pick after pick.
	first := runRound(t, exec, withKey("user-42"))
	assert.That(t, first != "").True()
	for range 5 {
		assert.String(t, runRound(t, exec, withKey("user-42"))).Equal(first)
	}

	// The keys spread across the ring: with three endpoints and many keys, a
	// single endpoint cannot hold every pick.
	seen := map[string]bool{}
	for i := range 30 {
		seen[runRound(t, exec, withKey(string(rune('a'+i%26))))] = true
	}
	assert.That(t, len(seen) > 1).True()

	// No key stated: the unkeyed fallback still answers.
	assert.That(t, runRound(t, exec, context.Background()) != "").True()
}
