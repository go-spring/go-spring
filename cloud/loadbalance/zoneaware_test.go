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
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestZoneAware(t *testing.T) {
	set := []discovery.Endpoint{
		{Addr: "a", Metadata: map[string]string{"zone": "z1"}},
		{Addr: "b", Metadata: map[string]string{"zone": "z1"}},
		{Addr: "c", Metadata: map[string]string{"zone": "z2"}},
	}
	b := NewZoneAware("zone", NewRoundRobin())

	// Requests from z1 stay in z1.
	m := counts(t, b, set, PickInfo{Zone: "z1"}, 20)
	assert.Number(t, m["c"]).Equal(0)
	assert.Number(t, m["a"]+m["b"]).Equal(20)

	// A zone with no local instances spills over to the whole set.
	m2 := counts(t, b, set, PickInfo{Zone: "z9"}, 3)
	assert.Number(t, m2["a"]+m2["b"]+m2["c"]).Equal(3)

	// No zone hint delegates over everything.
	m3 := counts(t, b, set, PickInfo{}, 3)
	assert.Number(t, m3["a"]+m3["b"]+m3["c"]).Equal(3)
}

func TestZoneAwareCompleteForwards(t *testing.T) {
	// Complete must reach the delegate so a stateful strategy composed under
	// zone_aware (least-conn here) keeps its per-request accounting.
	b := NewZoneAware("zone", NewLeastConn())
	set := []discovery.Endpoint{
		{Addr: "a", Metadata: map[string]string{"zone": "z1"}},
		{Addr: "b", Metadata: map[string]string{"zone": "z1"}},
	}
	e1, err := b.Pick(set, PickInfo{Zone: "z1"})
	assert.Error(t, err).Nil()
	e2, err := b.Pick(set, PickInfo{Zone: "z1"})
	assert.Error(t, err).Nil()
	assert.String(t, e1.Addr).NotEqual(e2.Addr) // in-flight count survived the wrap

	b.Complete(e1, nil)
	e3, err := b.Pick(set, PickInfo{Zone: "z1"})
	assert.Error(t, err).Nil()
	assert.String(t, e3.Addr).Equal(e1.Addr) // decrement reached the delegate
}

func TestZoneAwareLevels(t *testing.T) {
	set := []discovery.Endpoint{
		{Addr: "rack", Metadata: map[string]string{"zone": "cn-north-1a"}},
		{Addr: "az", Metadata: map[string]string{"zone": "cn-north-1b"}},
		{Addr: "region", Metadata: map[string]string{"zone": "cn-north-2"}},
	}
	b := NewZoneAware("zone", NewRoundRobin())

	// Exact first level wins: only the rack-local instance is picked.
	m := counts(t, b, set, PickInfo{Zone: "cn-north-1a"}, 5)
	assert.Number(t, m["rack"]).Equal(5)

	// Hierarchical fallback: the az-level hint matches same-AZ endpoints
	// (1a, 1b) but never the sibling region cn-north-2.
	m = counts(t, b, set, PickInfo{Zone: "cn-north-1"}, 6)
	assert.Number(t, m["rack"]+m["az"]).Equal(6)
	assert.Number(t, m["region"]).Equal(0)

	// Ordered multi-level: rack first; with the rack instance gone the az
	// level takes over; only when both are empty does it spill everywhere.
	subset := set[1:]
	m = counts(t, b, subset, PickInfo{Zone: "cn-north-1a,cn-north-1"}, 4)
	assert.Number(t, m["az"]).Equal(4)
	m = counts(t, b, subset, PickInfo{Zone: "cn-north-1a,cn-north-2"}, 4)
	assert.Number(t, m["region"]).Equal(4)
	m = counts(t, b, []discovery.Endpoint{set[2]}, PickInfo{Zone: "cn-north-1a,cn-north-1"}, 3)
	assert.Number(t, m["region"]).Equal(3)

	// All levels empty: full spill-over, never a black hole.
	m = counts(t, b, set, PickInfo{Zone: "cn-south-1,cn-south-2"}, 3)
	assert.Number(t, len(m)).Equal(3)

	// The classic single-zone usage is unchanged by the list support.
	m = counts(t, b, set, PickInfo{Zone: "cn-north-2"}, 3)
	assert.Number(t, m["region"]).Equal(3)
}

func TestZoneLevels(t *testing.T) {
	assert.That(t, len(parseZoneLevels(""))).Equal(0)
	assert.Slice(t, parseZoneLevels("cn-north-1a")).Length(1)
	assert.Slice(t, parseZoneLevels(" cn-north-1a , cn-north-1 ")).Length(2)
	assert.Slice(t, parseZoneLevels("a,,b")).Length(2)
	assert.That(t, zoneMatch("cn-north-1a", "cn-north-1a")).True()
	assert.That(t, zoneMatch("cn-north-1a", "cn-north-1")).True()
	assert.That(t, zoneMatch("cn-north-2", "cn-north-1")).False()
	assert.That(t, zoneMatch("cn-north-12", "cn-north-1")).True() // documented hierarchy edge
	assert.That(t, zoneMatch("", "cn-north-1")).False()
}
