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
	"cmp"
	"context"
	"math/rand/v2"
	"slices"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/hashutil"
)

const (
	// defaultReplicas is the number of virtual nodes each endpoint contributes
	// when the selection states none. It matches the tag's default.
	defaultReplicas = 100

	// maxReplicas bounds a selection's own setting. A governance document is
	// pushed at runtime, so an absurd value must be rejected rather than allowed
	// to allocate a gigantic ring on the request path: the tag enforces this for
	// bound documents, and the constructor clamps a code-built selection.
	maxReplicas = 1000

	// maxRingNodes bounds one ring's total node count, the way Envoy's ring hash
	// caps its ring size. Discovery can hand over thousands of endpoints, and it
	// is the product (endpoints × replicas) that allocates. Past the cap the
	// effective replica count shrinks, so minimal key movement is no longer
	// guaranteed exactly at that boundary; the cap is high enough that only
	// absurd topologies reach it.
	maxRingNodes = 1 << 20

	// maxCachedRings bounds how many recent rings are kept. The candidate set is
	// not always the same across calls — zone-aware balancing narrows it per
	// call — so alternating subsets would otherwise rebuild the ring on every
	// pick. A handful of recent rings covers the practical set of subsets.
	maxCachedRings = 4

	// maxCachedNodes bounds the total virtual nodes across cached rings, so the
	// cache cannot multiply memory past a small multiple of one full ring.
	maxCachedNodes = 2 * maxRingNodes
)

// NewConsistentHash builds the [ConsistentHash] strategy. Its only parameter is
// [Selection.ConsistentHashReplicas]: a non-positive value selects
// [defaultReplicas] (resolved when the ring is built, so the zero value works
// too), and a value above [maxReplicas] is clamped, keeping a code-built
// selection bounded just as the tag bounds a document.
func NewConsistentHash(sel Selection) *ConsistentHashBalancer {
	replicas := sel.ConsistentHashReplicas
	if replicas > maxReplicas {
		replicas = maxReplicas
	}
	b := &ConsistentHashBalancer{replicas: replicas}
	// The unkeyed round-robin fallback starts at a random offset so a fleet of
	// clients that starts together does not send its first unkeyed picks to the
	// same instance.
	b.fallbackSeq.Store(rand.Uint64())
	return b
}

// ConsistentHashBalancer is a consistent-hashing [Balancer]. Requests carrying
// the same [PickInfo.HashKey] map to the same endpoint while the topology is
// stable, and only a small fraction of keys move when instances are added or
// removed — useful for cache affinity or sticky sessions.
//
// Requests with an empty HashKey fall back to round-robin, so unkeyed traffic
// still spreads instead of hammering one instance.
//
// The ring caches the ADDRESS SET, not endpoint snapshots: a pick always returns
// the endpoint from its own eps, so metadata, scheme or weight may change
// without moving keys. A ring is built only when its set's fingerprint is not
// among the recently cached ones, and a rebuilt ring is published whole, so a
// pick is one atomic load with no lock and no per-request allocation on a hit.
// Several recent rings are kept because the candidate set can alternate between
// calls (zone-aware balancing narrows it per call); a single-ring cache would
// rebuild on every alternate pick.
//
// The first keyed pick after a set change builds the ring on the request path:
// the discovery resolver exposes no change notification, so the balancer detects
// movement itself — at the cost of one hash per endpoint on every keyed pick —
// and pays the build (milliseconds for a large ring) on the pick that first sees
// it.
//
// [discovery.Endpoint.Weight] is not taken into account: the ring is built from
// addresses only, and every endpoint is treated equally. Weight belongs to the
// weighted strategy; consistent hashing here is not weighted.
type ConsistentHashBalancer struct {
	// replicas is the configured virtual-node count per endpoint, already
	// bounded by [maxReplicas]. A non-positive value means [defaultReplicas],
	// which buildRing resolves — that is what keeps the zero value usable.
	replicas int

	// fallbackSeq is the round-robin cursor used when a pick carries no HashKey.
	fallbackSeq atomic.Uint64

	// rings is the published cache of recent ring generations, immutable and
	// replaced whole. Readers load it once; builders replace it under buildMu.
	rings atomic.Pointer[ringCache]

	// buildMu serializes ring builds. The read path never takes it: a pick loads
	// the published snapshot atomically, and only goroutines that see a stale
	// fingerprint contend here. Without it, a set change would make every
	// in-flight pick build its own copy of the ring before one of them wins.
	buildMu sync.Mutex
}

// ringSnapshot is one immutable ring. It is built on demand and published whole,
// so concurrent picks share it without locking.
type ringSnapshot struct {
	fingerprint ringFingerprint
	nodes       []ringNode
}

// ringCache is the immutable set of recently built rings, newest first. It is
// replaced whole on a miss, so readers can hold a loaded reference without
// locking. The bounds keep a busy, churning balancer from retaining rings for
// every subset it has ever seen.
type ringCache struct {
	snapshots []*ringSnapshot
	nodes     int // total virtual nodes across snapshots, for the memory bound
}

// find returns the ring built for fp, or nil.
func (c *ringCache) find(fp ringFingerprint) *ringSnapshot {
	if c == nil {
		return nil
	}
	for _, s := range c.snapshots {
		if s.fingerprint == fp {
			return s
		}
	}
	return nil
}

// add returns a new cache with s in front, dropping the oldest entries until the
// ring-count and node-count bounds hold.
func (c *ringCache) add(s *ringSnapshot) *ringCache {
	var keep []*ringSnapshot
	if c != nil {
		keep = c.snapshots
	}
	snapshots := make([]*ringSnapshot, 0, len(keep)+1)
	snapshots = append(snapshots, s)
	nodes := len(s.nodes)
	for _, old := range keep {
		if old.fingerprint == s.fingerprint {
			continue
		}
		snapshots = append(snapshots, old)
		nodes += len(old.nodes)
	}
	for len(snapshots) > maxCachedRings || nodes > maxCachedNodes {
		if len(snapshots) <= 1 {
			break
		}
		last := snapshots[len(snapshots)-1]
		nodes -= len(last.nodes)
		snapshots = snapshots[:len(snapshots)-1]
	}
	return &ringCache{snapshots: snapshots, nodes: nodes}
}

// ringNode is one virtual node: where it sits on the ring and which endpoint
// owns it. Only the address is kept — the ring caches the address set, while the
// Endpoint itself is read fresh from every Pick's eps.
type ringNode struct {
	point uint32
	addr  string
}

// Pick selects one endpoint for the request. A non-empty HashKey maps through
// the ring, so the same key lands on the same endpoint while the topology is
// stable; an empty HashKey falls back to round-robin over eps.
//
// The returned endpoint comes from eps — this pick's snapshot — because the ring
// only chooses an address. eps is read, never mutated.
func (b *ConsistentHashBalancer) Pick(eps []discovery.Endpoint, info PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}
	if info.HashKey == "" {
		i := b.fallbackSeq.Add(1) - 1
		return eps[int(i%uint64(len(eps)))], nil
	}

	nodes := b.ringFor(eps)
	h := hashutil.FNV1a32(info.HashKey)
	// First point on the ring at or after h, wrapping around to the start.
	i := sort.Search(len(nodes), func(i int) bool { return nodes[i].point >= h })
	if i == len(nodes) {
		i = 0
	}
	addr := nodes[i].addr

	for _, ep := range eps {
		if ep.Addr == addr {
			return ep, nil
		}
	}
	// The fingerprint said the set was unchanged, so addr should be present; a
	// miss means a fingerprint collision, and any live endpoint is a safe answer.
	return eps[0], nil
}

// Complete is a no-op: the ring cache is rebuilt from Pick inputs, not outcomes.
func (b *ConsistentHashBalancer) Complete(_ context.Context, _ discovery.Endpoint, _ error) {}

// ringFor returns the ring for eps, building and caching one when the address
// set is not among the recently built ones. The published cache is immutable,
// so readers need no lock; only the build is serialized, and a goroutine that
// waited for another build reuses it when its fingerprint matches.
func (b *ConsistentHashBalancer) ringFor(eps []discovery.Endpoint) []ringNode {
	fp := fingerprintOf(eps)
	if s := b.rings.Load().find(fp); s != nil {
		return s.nodes
	}

	// One builder at a time: a set change makes every in-flight pick miss, and
	// letting each build its own ring would duplicate the (possibly
	// maxRingNodes-sized) allocation.
	b.buildMu.Lock()
	defer b.buildMu.Unlock()
	if s := b.rings.Load().find(fp); s != nil {
		return s.nodes
	}
	s := b.buildRing(eps, fp)
	b.rings.Store(b.rings.Load().add(s))
	return s.nodes
}

// buildRing lays out the virtual nodes for eps and sorts them by ring position:
// each endpoint contributes replicas nodes at hash(addr#replica). It holds no
// lock — the result is immutable and published whole by the caller.
func (b *ConsistentHashBalancer) buildRing(eps []discovery.Endpoint, fingerprint ringFingerprint) *ringSnapshot {
	replicas := b.replicas
	if replicas <= 0 {
		replicas = defaultReplicas
	}
	if replicas > maxRingNodes/len(eps) {
		replicas = maxRingNodes / len(eps)
		if replicas < 1 {
			replicas = 1
		}
	}

	nodes := make([]ringNode, 0, len(eps)*replicas)
	buf := make([]byte, 0, 64)
	for _, ep := range eps {
		for i := 0; i < replicas; i++ {
			buf = buf[:0]
			buf = append(buf, ep.Addr...)
			buf = append(buf, '#')
			buf = strconv.AppendInt(buf, int64(i), 10)
			// Keep the nodes side by side: two virtual nodes of different
			// endpoints may hash to the same point, and a map keyed by point
			// would drop one of them.
			nodes = append(nodes, ringNode{point: hashutil.FNV1a32Bytes(buf), addr: ep.Addr})
		}
	}
	slices.SortFunc(nodes, func(a, b ringNode) int { return cmp.Compare(a.point, b.point) })
	return &ringSnapshot{fingerprint: fingerprint, nodes: nodes}
}

// ringFingerprint is an order-independent digest of the addresses in eps, used
// to detect when the ring must be rebuilt. XOR of per-address hashes is
// commutative, so a reordered but otherwise identical set keeps the same ring;
// n guards the XOR's one blind spot (two addresses hashing identically cancel
// out). Being a comparable value rather than a string, it costs a keyed pick
// nothing to test.
type ringFingerprint struct {
	sum uint64
	n   int
}

// fingerprintOf digests the address set of eps for change detection.
func fingerprintOf(eps []discovery.Endpoint) ringFingerprint {
	var sum uint64
	for _, ep := range eps {
		sum ^= uint64(hashutil.FNV1a32(ep.Addr))
	}
	return ringFingerprint{sum: sum, n: len(eps)}
}
