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
	"strings"

	"go-spring.org/cloud/discovery"
)

// DefaultZoneKey is the [discovery.Endpoint.Metadata] key a zone-aware balancer
// reads to learn an instance's locality when none is given explicitly.
const DefaultZoneKey = "zone"

func init() {
	Register(ZoneAware, func() Balancer {
		return NewZoneAware(DefaultZoneKey, NewRoundRobin())
	})
}

// zoneAware is a filter, not a strategy: it narrows the candidate set to the
// best-matching zone level and delegates the actual choice, so it composes
// with any [Balancer] (round-robin inside the zone, least-conn, ...).
type zoneAware struct {
	zoneKey  string
	delegate Balancer
}

// NewZoneAware returns a locality-aware [Balancer]. It prefers endpoints whose
// Metadata[zoneKey] matches the caller's [PickInfo.Zone], delegating the final
// choice among the local subset to delegate. When the caller advertises no zone,
// or no endpoint matches any of its levels, the balancer spills over to the
// full set so traffic is never black-holed just because the local zones are
// empty.
//
// PickInfo.Zone may carry a single zone ("cn-north-1a" — the classic usage) or
// an ordered fallback list ("cn-north-1a,cn-north-1": my rack, then my zone).
// Levels are tried left to right and the first level with at least one match
// wins; see [parseZoneLevels] for how a level matches an endpoint.
//
// This keeps traffic in-zone (lower latency, no cross-zone egress cost) under
// normal conditions while degrading gracefully during a zonal outage.
func NewZoneAware(zoneKey string, delegate Balancer) Balancer {
	if zoneKey == "" {
		zoneKey = DefaultZoneKey
	}
	return &zoneAware{zoneKey: zoneKey, delegate: delegate}
}

func (b *zoneAware) Pick(eps []discovery.Endpoint, info PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoAvailable
	}
	levels := parseZoneLevels(info.Zone)
	if len(levels) == 0 {
		return b.delegate.Pick(eps, info)
	}

	for _, level := range levels {
		matched := eps[:0:0]
		for _, ep := range eps {
			if zoneMatch(ep.Metadata[b.zoneKey], level) {
				matched = append(matched, ep)
			}
		}
		if len(matched) > 0 {
			return b.delegate.Pick(matched, info)
		}
	}
	// No instance in any of the caller's zone levels: spill over to every
	// endpoint rather than fail, trading locality for availability.
	return b.delegate.Pick(eps, info)
}

// Complete forwards to the delegate so strategies composed under zone_aware
// keep their per-request accounting.
func (b *zoneAware) Complete(ep discovery.Endpoint, err error) {
	b.delegate.Complete(ep, err)
}

// parseZoneLevels parses the caller's zone hint into an ordered fallback list: a
// comma-separated string yields its non-empty trimmed parts in order, anything
// else yields the single value (possibly none when empty).
func parseZoneLevels(zone string) []string {
	if zone == "" {
		return nil
	}
	parts := strings.Split(zone, ",")
	levels := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			levels = append(levels, v)
		}
	}
	return levels
}

// zoneMatch reports whether an endpoint's zone satisfies a fallback level. A
// level matches on exact equality ("cn-north-1a" == "cn-north-1a") or when the
// endpoint's zone extends it: level "cn-north-1" also matches endpoint
// "cn-north-1a", so a zone-level hint keeps traffic inside the availability
// zone before spilling region-wide. Sibling names never match ("cn-north-1"
// does not match "cn-north-2"); an unlucky name that merely shares the prefix
// ("cn-north-12") does — acceptable for the hierarchical naming schemes this
// targets, since the ordered list lets callers fall through anyway.
func zoneMatch(epZone, level string) bool {
	return epZone == level || strings.HasPrefix(epZone, level)
}
