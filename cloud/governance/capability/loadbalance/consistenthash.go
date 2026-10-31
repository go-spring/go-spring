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
	"slices"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/stdlib/hashutil"
)

// consistentHashFactory is the [Factory] behind the [ConsistentHash] name. Its
// only parameter is [ConsistentHashParams.Replicas].
func init() {
	Register(ConsistentHash, newConsistentHash)
}

// ConsistentHashParams is the consistent-hash strategy's parameter group, bound
// from a rule's `consistent-hash` sub-map.
type ConsistentHashParams struct {
	// Replicas is the number of virtual nodes per endpoint. A rule that omits
	// the key gets the default; a caller building a Selection in code states a
	// positive count.
	Replicas int `value:"${replicas:=100}" expr:"$ > 0"`
}

func newConsistentHash(sel Selection) (Balancer, error) {
	// The tag owns this field's validity: it supplies 100 when the rule says
	// nothing, and its expr rejects anything non-positive.
	return &consistentHash{
		replicas: sel.ConsistentHash.Replicas,
	}, nil
}

// consistentHash is a consistent-hashing [Balancer]. Requests carrying the same
// [PickInfo.HashKey] map to the same endpoint as long as the topology is stable,
// and only a small fraction of keys move when instances are added or removed —
// useful for cache affinity or sticky sessions.
//
// Requests with an empty HashKey fall back to round-robin so the balancer still
// spreads unkeyed traffic.
//
// It keeps a hash ring cached against the current endpoint set.
// The ring is the expensive part (replicas × endpoints entries), so it is
// rebuilt only when the set's fingerprint changes instead of on every pick.
type consistentHash struct {
	replicas int

	rr atomic.Uint64 // fallback cursor for empty HashKey

	mu    sync.Mutex
	fp    string // fingerprint of the endpoint set the ring was built from
	ring  []uint32
	owner map[uint32]discovery.Endpoint
}

func (b *consistentHash) Pick(eps []discovery.Endpoint, info PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}
	if info.HashKey == "" {
		// No key to hash on: behave like round-robin so unkeyed traffic still
		// spreads instead of hammering one instance.
		i := b.rr.Add(1) - 1
		return eps[int(i%uint64(len(eps)))], nil
	}

	b.mu.Lock()
	b.rebuild(eps)
	h := hashutil.FNV1a32(info.HashKey)
	// First point on the ring at or after h, wrapping around to the start.
	i := sort.Search(len(b.ring), func(i int) bool { return b.ring[i] >= h })
	if i == len(b.ring) {
		i = 0
	}
	ep := b.owner[b.ring[i]]
	b.mu.Unlock()
	return ep, nil
}

// Complete is a no-op: the ring cache is rebuilt from Pick inputs, not outcomes.
func (b *consistentHash) Complete(_ context.Context, _ discovery.Endpoint, _ error) {}

// rebuild reconstructs the ring only when the endpoint set changed, keyed by a
// cheap order-independent fingerprint. Caller holds b.mu.
func (b *consistentHash) rebuild(eps []discovery.Endpoint) {
	fp := fingerprint(eps)
	if fp == b.fp && b.ring != nil {
		return
	}
	ring := make([]uint32, 0, len(eps)*b.replicas)
	owner := make(map[uint32]discovery.Endpoint, len(eps)*b.replicas)
	for _, ep := range eps {
		for i := 0; i < b.replicas; i++ {
			point := hashutil.FNV1a32(ep.Addr + "#" + strconv.Itoa(i))
			ring = append(ring, point)
			owner[point] = ep
		}
	}
	slices.Sort(ring)
	b.ring = ring
	b.owner = owner
	b.fp = fp
}

// fingerprint is an order-independent digest of the addresses in eps, used to
// detect when the ring must be rebuilt. XOR of per-address hashes is
// commutative, so a reordered but otherwise identical set keeps the same ring.
func fingerprint(eps []discovery.Endpoint) string {
	var x uint64
	for _, ep := range eps {
		x ^= uint64(hashutil.FNV1a32(ep.Addr))
	}
	return strconv.FormatUint(x, 16) + ":" + strconv.Itoa(len(eps))
}
