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
	"hash/fnv"
	"slices"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"go-spring.org/stdlib/discovery"
)

// defaultReplicas is the number of virtual nodes placed on the ring per
// endpoint. More replicas smooth out the key distribution at the cost of a
// larger ring; 100 is a common, balanced default.
const defaultReplicas = 100

func init() {
	Register(ConsistentHash, func() Balancer { return NewConsistentHash(defaultReplicas) })
}

// NewConsistentHash returns a consistent-hashing [Balancer]. Requests carrying
// the same [PickInfo.HashKey] map to the same endpoint as long as the topology
// is stable, and only a small fraction of keys move when instances are added or
// removed — useful for cache affinity or sticky sessions.
//
// replicas is the number of virtual nodes per endpoint (<=0 uses the default).
// Requests with an empty HashKey fall back to round-robin so the balancer still
// spreads unkeyed traffic.
func NewConsistentHash(replicas int) Balancer {
	if replicas <= 0 {
		replicas = defaultReplicas
	}
	return &consistentHash{replicas: replicas}
}

type consistentHash struct {
	replicas int

	rr atomic.Uint64 // fallback cursor for empty HashKey

	mu    sync.Mutex
	fp    string  // fingerprint of the endpoint set the ring was built from
	ring  []uint32
	owner map[uint32]discovery.Endpoint
}

func (b *consistentHash) Pick(eps []discovery.Endpoint, info PickInfo) (Result, error) {
	if len(eps) == 0 {
		return Result{}, ErrNoAvailable
	}
	if info.HashKey == "" {
		// No key to hash on: behave like round-robin so unkeyed traffic still
		// spreads instead of hammering one instance.
		i := b.rr.Add(1) - 1
		return Result{Endpoint: eps[int(i%uint64(len(eps)))]}, nil
	}

	b.mu.Lock()
	b.rebuild(eps)
	h := hashKey(info.HashKey)
	// First point on the ring at or after h, wrapping around to the start.
	i := sort.Search(len(b.ring), func(i int) bool { return b.ring[i] >= h })
	if i == len(b.ring) {
		i = 0
	}
	ep := b.owner[b.ring[i]]
	b.mu.Unlock()
	return Result{Endpoint: ep}, nil
}

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
			point := hashKey(ep.Addr + "#" + strconv.Itoa(i))
			ring = append(ring, point)
			owner[point] = ep
		}
	}
	slices.Sort(ring)
	b.ring = ring
	b.owner = owner
	b.fp = fp
}

func hashKey(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

// fingerprint is an order-independent digest of the addresses in eps, used to
// detect when the ring must be rebuilt. XOR of per-address hashes is
// commutative, so a reordered but otherwise identical set keeps the same ring.
func fingerprint(eps []discovery.Endpoint) string {
	var x uint64
	for _, ep := range eps {
		x ^= uint64(hashKey(ep.Addr))
	}
	return strconv.FormatUint(x, 16) + ":" + strconv.Itoa(len(eps))
}
