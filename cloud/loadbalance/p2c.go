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
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"go-spring.org/cloud/discovery"
)

func init() {
	Register(P2C, NewP2C)
}

// ewmaBeta is the weight of each new sample in the latency moving average.
// 0.2 adapts within a handful of requests while tolerating single outliers.
const ewmaBeta = 0.2

// p2cFailurePenalty is the latency a failed request contributes to the EWMA.
// One second is deliberately larger than any healthy round-trip, so failures
// dominate the score much faster than slow successes do.
const p2cFailurePenalty = time.Second

// ewmaTau is the staleness half-life of the latency model: an address that
// stops being picked sees its stored EWMA decay at read time, so a momentarily
// slow instance re-enters rotation instead of starving behind a frozen
// estimate (the same aging gRPC's weighted round robin applies).
const ewmaTau = 10 * time.Second

// p2c keeps per-address state keyed by Addr so the candidate set can change
// underneath without losing the latency model for surviving addresses.
type p2c struct {
	mu sync.Mutex
	// inflight counts outstanding picks per address (the load half of the score).
	inflight map[string]int
	// ewma is each address's exponentially-weighted moving average of request
	// duration in nanoseconds (the latency half; see ewmaBeta).
	ewma map[string]float64
	// last is when each address's EWMA was last updated, driving the
	// staleness aging in score (see ewmaTau).
	last map[string]time.Time
	// started holds the Pick timestamps of outstanding requests per address,
	// FIFO: Complete pops the oldest outstanding start for that address — an
	// approximation of per-request timing that the two-method API makes exact
	// enough for a moving average.
	started map[string][]time.Time

	// now is the clock, injectable so tests can drive the staleness aging
	// deterministically (same idiom as Tracker). Defaults to time.Now.
	now func() time.Time
}

// NewP2C returns a power-of-two-choices [Balancer], the strategy gRPC,
// Finagle and Dubbo converge on: each pick draws two random candidates and
// routes to the one with the lower cost score, exponentially improving on
// uniform random while never scanning the whole set. The score combines an
// EWMA of observed request latency with the in-flight count
// (score = ewma * (inflight + 1)), so a slow-but-idle instance still loses to
// a fast-but-busy one only when the math says so.
//
// Latency is learned from [Balancer.Complete]: the duration between Pick and
// Complete is fed into a per-address exponential moving average, and a failed
// request contributes a fixed penalty so failures push the instance out
// quickly. Callers must therefore pair every Pick with a Complete, as the
// interface contract requires — with p2c the pairing is what keeps the
// latency model honest.
func NewP2C() Balancer {
	return &p2c{
		inflight: map[string]int{},
		ewma:     map[string]float64{},
		last:     map[string]time.Time{},
		started:  map[string][]time.Time{},
		now:      time.Now,
	}
}

func (b *p2c) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoAvailable
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
	best := eps[i]
	if j >= 0 && b.score(eps[j].Addr) < b.score(best.Addr) {
		best = eps[j]
	}
	b.inflight[best.Addr]++
	b.started[best.Addr] = append(b.started[best.Addr], b.now())
	b.mu.Unlock()
	return best, nil
}

// Complete pops the oldest outstanding Pick timestamp for ep.Addr, feeds the
// observed duration into the latency EWMA (or the failure penalty when err is
// non-nil), and releases the in-flight slot.
func (b *p2c) Complete(ep discovery.Endpoint, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	starts := b.started[ep.Addr]
	if len(starts) > 0 {
		d := b.now().Sub(starts[0])
		b.started[ep.Addr] = starts[1:]
		if err != nil {
			d = p2cFailurePenalty
		}
		old := b.ewma[ep.Addr]
		b.ewma[ep.Addr] = old + ewmaBeta*(float64(d)-old)
		b.last[ep.Addr] = b.now()
	}
	if n := b.inflight[ep.Addr]; n <= 1 {
		delete(b.inflight, ep.Addr)
	} else {
		b.inflight[ep.Addr] = n - 1
	}
}

// score ranks an address for the two-choices comparison: estimated request
// cost scaled by how busy the address already is, with the stored EWMA decayed
// by the time since it was last observed (staleness aging). An address with no
// latency history scores 0 and is preferred until it has data — the
// exploration phase that seeds the model.
func (b *p2c) score(addr string) float64 {
	e := b.ewma[addr]
	if t, ok := b.last[addr]; ok {
		e *= math.Exp(-float64(b.now().Sub(t)) / float64(ewmaTau))
	}
	return e * float64(b.inflight[addr]+1)
}
