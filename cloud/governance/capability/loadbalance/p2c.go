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
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"go-spring.org/cloud/discovery"
)

// NewP2C builds the [P2C] strategy. It has no parameters of its own.
func NewP2C(Selection) *P2CBalancer {
	return &P2CBalancer{
		inflight:   map[string]int{},
		ewma:       map[string]float64{},
		observedAt: map[string]time.Time{},
		startTimes: map[string][]time.Time{},
	}
}

// ewmaBeta is the weight of each new sample in the latency moving average.
// 0.2 adapts within a handful of requests while tolerating single outliers.
const ewmaBeta = 0.2

// ewmaTau is the staleness half-life of the latency model: an address that
// stops being picked sees its stored EWMA decay at read time, so a momentarily
// slow instance re-enters rotation instead of starving behind a frozen
// estimate (the same aging gRPC's weighted round robin applies).
const ewmaTau = 10 * time.Second

// initialLatency is the cost assumed for an address that has not completed a
// request yet. It is deliberately pessimistic — a freshly discovered instance
// must not win every comparison and absorb a burst before its first measurement,
// while the in-flight factor still applies to it. The prior decays with ewmaTau
// like a real estimate, so the instance is explored once it has aged; this is the
// same shape as Linkerd's EWMA_DEFAULT_RTT (30ms).
const initialLatency = 30 * time.Millisecond

// minLatency floors a decayed estimate (one nanosecond). Without it, an address
// idle for hours underflows the exponential to zero, and multiplying by the
// in-flight count would again erase the load factor.
const minLatency = 1.0

// P2CBalancer is a power-of-two-choices [Balancer], the strategy gRPC, Finagle and
// Dubbo converge on: each pick draws two random candidates and routes to the one
// with the lower cost score, exponentially improving on uniform random while
// never scanning the whole set. The score combines an EWMA of observed request
// latency with the in-flight count (score = ewma * (inflight + 1)), so a
// slow-but-idle instance still loses to a fast-but-busy one only when the math
// says so.
//
// Latency is learned from [Balancer.Complete]: the duration of a SUCCESSFUL
// request is fed into a per-address exponential moving average. A failed request
// is not sampled — its duration says nothing about how fast the instance is, and
// turning it into a synthetic penalty would duplicate the [Tracker]'s failure
// handling while trapping the peer: a penalised address is not selected, so it
// could only recover by decay. Callers must pair every Pick with a Complete, as
// the interface contract requires — with p2c the pairing is what keeps the
// latency model honest.
//
// It keeps per-address state keyed by Addr so the candidate set can change
// underneath without losing the latency model for surviving addresses. An
// address with no measurement yet is scored from [initialLatency], not from zero,
// so its in-flight count matters from the first pick; the prior decays over time,
// which keeps exploration alive. State for addresses that leave the candidate set
// is pruned (see [P2CBalancer.prune]), so instance churn does not grow the tables
// without bound.
//
// [discovery.Endpoint.Weight] is not taken into account: the score is driven by
// latency and in-flight count alone. Weight belongs to the weighted strategy.
//
// Build it with [NewP2C]: the tables are not created lazily.
type P2CBalancer struct {
	mu sync.Mutex
	// inflight counts outstanding picks per address (the load half of the score).
	inflight map[string]int
	// ewma is each address's exponentially-weighted moving average of request
	// duration in nanoseconds (the latency half; see ewmaBeta).
	ewma map[string]float64
	// observedAt is when each address's estimate was last refreshed (a successful
	// sample or the initial prior), driving the staleness decay in score.
	observedAt map[string]time.Time
	// startTimes holds the Pick timestamps of outstanding requests per address,
	// FIFO: Complete pops the oldest outstanding start for that address — an
	// approximation of per-request timing that the two-method API makes exact
	// enough for a moving average.
	startTimes map[string][]time.Time
	// prunedAt is when prune last ran, so a shrunken candidate set cannot make
	// every pick pay for a full scan: at most one sweep per [ewmaTau].
	prunedAt time.Time
}

// Pick draws two distinct random candidates and returns the one with the lower
// score; with a single candidate it returns that one. eps is read, never mutated.
func (b *P2CBalancer) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}

	// Two distinct random indices: i uniform over the set, j uniform over the
	// rest (the shift keeps both marginals uniform).
	i := rand.IntN(len(eps))
	j := -1
	if len(eps) > 1 {
		j = rand.IntN(len(eps) - 1)
		if j >= i {
			j++
		}
	}

	b.mu.Lock()
	// The sweep is amortized and cooldown-bounded: a shrunken candidate set keeps
	// the ratio over the threshold while recently observed entries are retained,
	// so without the cooldown every pick would rescan the table.
	if len(b.observedAt) > 2*len(eps) && time.Since(b.prunedAt) >= ewmaTau {
		b.prune(eps)
		b.prunedAt = time.Now()
	}
	best := eps[i]
	if j >= 0 && b.score(eps[j].Addr) < b.score(best.Addr) {
		best = eps[j]
	}
	b.inflight[best.Addr]++
	b.startTimes[best.Addr] = append(b.startTimes[best.Addr], time.Now())
	b.mu.Unlock()
	return best, nil
}

// Complete pops the oldest outstanding Pick timestamp for ep.Addr, feeds the
// observed duration of a successful request into the latency EWMA, and releases
// the in-flight slot. A failed request releases its slot without sampling: the
// latency model describes how fast a healthy instance is, and repeated failures
// are what the [Tracker] suspends on.
func (b *P2CBalancer) Complete(_ context.Context, ep discovery.Endpoint, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	starts := b.startTimes[ep.Addr]
	if len(starts) > 0 {
		start := starts[0]
		b.startTimes[ep.Addr] = starts[1:]
		if err == nil {
			d := time.Since(start)
			old := b.ewma[ep.Addr]
			b.ewma[ep.Addr] = old + ewmaBeta*(float64(d)-old)
			b.observedAt[ep.Addr] = time.Now()
		}
	}
	if n := b.inflight[ep.Addr]; n <= 1 {
		delete(b.inflight, ep.Addr)
	} else {
		b.inflight[ep.Addr] = n - 1
	}
}

// score ranks an address for the two-choices comparison: estimated request cost
// scaled by how busy it is, with the stored estimate decayed by the time since it
// was last observed. An address with no measurement yet is seeded from
// [initialLatency] so the in-flight factor applies from the first pick; the seed
// decays like any estimate, which is what eventually makes the instance
// explorable. Caller holds b.mu.
func (b *P2CBalancer) score(addr string) float64 {
	e, measured := b.ewma[addr]
	if !measured {
		e = float64(initialLatency)
		b.ewma[addr] = e
		b.observedAt[addr] = time.Now()
	} else if t, ok := b.observedAt[addr]; ok {
		e *= math.Exp(-float64(time.Since(t)) / float64(ewmaTau))
		if e < minLatency {
			e = minLatency
		}
	}
	return e * float64(b.inflight[addr]+1)
}

// prune drops latency state for addresses that have left the candidate set,
// keeping the tables bounded across instance churn. It runs only when the table
// has grown past twice the live set, so the scan is amortized. An address with
// requests in flight, or last observed within the staleness window, keeps its
// state: a temporary suspension must not erase a peer's history. Caller holds
// b.mu.
func (b *P2CBalancer) prune(eps []discovery.Endpoint) {
	live := make(map[string]struct{}, len(eps))
	for _, ep := range eps {
		live[ep.Addr] = struct{}{}
	}
	now := time.Now()
	for addr, observed := range b.observedAt {
		if _, ok := live[addr]; ok {
			continue
		}
		if b.inflight[addr] != 0 || now.Sub(observed) < ewmaTau {
			continue
		}
		delete(b.ewma, addr)
		delete(b.observedAt, addr)
		delete(b.startTimes, addr)
	}
}
