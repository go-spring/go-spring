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
	"go-spring.org/spring/discovery"
)

// ZoneAware is the registered name of the default zone-aware strategy (local
// zone preference over a round-robin delegate, reading the "zone" metadata key).
const ZoneAware = "zone_aware"

// DefaultZoneKey is the [discovery.Endpoint.Metadata] key a zone-aware balancer
// reads to learn an instance's locality when none is given explicitly.
const DefaultZoneKey = "zone"

func init() {
	Register(ZoneAware, func() Balancer {
		return NewZoneAware(DefaultZoneKey, NewRoundRobin())
	})
}

// NewZoneAware returns a locality-aware [Balancer]. It prefers endpoints whose
// Metadata[zoneKey] equals the caller's [PickInfo.Zone], delegating the final
// choice among the local subset to delegate. When the caller advertises no zone,
// or no endpoint matches it, the balancer spills over to the full set so traffic
// is never black-holed just because the local zone is empty.
//
// This keeps traffic in-zone (lower latency, no cross-zone egress cost) under
// normal conditions while degrading gracefully during a zonal outage.
func NewZoneAware(zoneKey string, delegate Balancer) Balancer {
	if zoneKey == "" {
		zoneKey = DefaultZoneKey
	}
	return &zoneAware{zoneKey: zoneKey, delegate: delegate}
}

type zoneAware struct {
	zoneKey  string
	delegate Balancer
}

func (b *zoneAware) Pick(eps []discovery.Endpoint, info PickInfo) (Result, error) {
	if len(eps) == 0 {
		return Result{}, ErrNoAvailable
	}
	if info.Zone == "" {
		return b.delegate.Pick(eps, info)
	}

	local := eps[:0:0]
	for _, ep := range eps {
		if ep.Metadata[b.zoneKey] == info.Zone {
			local = append(local, ep)
		}
	}
	if len(local) == 0 {
		// No instance in the caller's zone: spill over to every endpoint rather
		// than fail, trading locality for availability.
		return b.delegate.Pick(eps, info)
	}
	return b.delegate.Pick(local, info)
}
