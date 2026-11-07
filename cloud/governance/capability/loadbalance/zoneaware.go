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
	"strings"

	"go-spring.org/cloud/discovery"
)

// NewZoneAware wraps delegate so picks prefer endpoints in the caller's zone.
// An empty zoneKey falls back to [discovery.MetaKeyZone]. It is a filter, not a
// strategy: [Factory.Create] applies it after building the strategy the
// selection names, which is what lets a rule state its balancer only once.
func NewZoneAware(delegate Balancer, zoneKey string) *ZoneAwareBalancer {
	if delegate == nil {
		panic("loadbalance: NewZoneAware requires a non-nil delegate")
	}
	if zoneKey == "" {
		zoneKey = discovery.MetaKeyZone
	}
	return &ZoneAwareBalancer{zoneKey: zoneKey, delegate: delegate}
}

// ZoneAwareBalancer is a locality-aware [Balancer] — a filter rather than a
// strategy: it narrows the candidate set to the best-matching zone level and
// delegates the actual choice, so it composes with any [Balancer] (round-robin
// inside the zone, least-conn, ...).
//
// It prefers endpoints whose Metadata[zoneKey] matches the caller's
// [PickInfo.Zone]. When the caller advertises no zone, or no endpoint matches any
// of its levels, it spills over to the full set so traffic is never black-holed
// just because the local zones are empty.
//
// PickInfo.Zone may carry a single zone ("cn-north-1a" — the classic usage) or
// an ordered fallback list ("cn-north-1a,cn-north-1": my rack, then my zone).
// Levels are tried left to right and the first level with at least one match
// wins; see [matchesZone] for how a level matches an endpoint.
//
// This keeps traffic in-zone (lower latency, no cross-zone egress cost) under
// normal conditions while degrading gracefully during a zonal outage.
//
// Build it with [NewZoneAware]: an empty zoneKey means [discovery.MetaKeyZone],
// and the delegate must be non-nil.
type ZoneAwareBalancer struct {
	// zoneKey is the endpoint metadata key locality is read from.
	zoneKey string

	// delegate makes the actual choice among the endpoints of the matched level.
	delegate Balancer
}

// Pick narrows eps to the best-matching zone level and delegates the choice; see
// the type doc for the fallback order. It returns [ErrNoEndpoints] when eps is
// empty.
func (b *ZoneAwareBalancer) Pick(eps []discovery.Endpoint, info PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}
	levels := splitZoneLevels(info.Zone)
	if len(levels) == 0 {
		return b.delegate.Pick(eps, info)
	}

	// Levels are alternatives, not a union: the first level with any match wins
	// and the request is balanced only within that level's endpoints, so a
	// caller whose nearest level is non-empty never reaches a farther one. The
	// next level is considered only when the previous one has no endpoint at
	// all, and the full set is used only when every level is empty. Merging the
	// levels instead would dilute locality — a rack-local caller would share
	// traffic with zone- and region-wide instances.
	for _, level := range levels {
		matched := make([]discovery.Endpoint, 0, len(eps))
		for _, ep := range eps {
			if matchesZone(ep.Metadata[b.zoneKey], level) {
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

// Complete forwards to the delegate so strategies wrapped by the zone-aware
// filter keep their per-request accounting.
func (b *ZoneAwareBalancer) Complete(ctx context.Context, ep discovery.Endpoint, err error) {
	b.delegate.Complete(ctx, ep, err)
}

// splitZoneLevels splits the caller's zone hint into an ordered fallback list: a
// comma-separated string yields its non-empty trimmed parts in order, anything
// else yields the single value (possibly none when empty).
func splitZoneLevels(zone string) []string {
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

// matchesZone reports whether an endpoint's zone satisfies a fallback level. A
// level matches on exact equality ("cn-north-1a" == "cn-north-1a") or when the
// endpoint's zone extends it on a COMPONENT boundary: level "cn-north-1" matches
// "cn-north-1a" (an availability zone inside the region) and "cn-north" matches
// "cn-north-1" (the separator does the work). A prefix followed by a DIGIT is a
// different numeric component, not a child: "cn-north-1" must not match
// "cn-north-10" or "cn-north-10a", whose region is 10. Without that rule a
// caller in region 1 would treat region 10 as local and defeat the fallback.
func matchesZone(epZone, level string) bool {
	if epZone == level {
		return true
	}
	if !strings.HasPrefix(epZone, level) {
		return false
	}
	// The prefix is proper here (the equality case returned above), so the byte
	// after it always exists.
	next := epZone[len(level)]
	return next < '0' || next > '9'
}
