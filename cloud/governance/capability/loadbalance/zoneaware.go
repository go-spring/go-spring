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

	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/stdlib/errutil"
)

func init() {
	Register(ZoneAware, newZoneAware)
}

// ZoneAwareParams is the zone-aware filter's parameter group, bound from a
// rule's `zone-aware` sub-map. A rule that omits zone-key gets
// [discovery.MetaKeyZone] from the tag, and an explicit empty value is refused
// there by the expr. The delegate rules are conditional or identity-bound, so
// the factory states them instead (see [newZoneAware]).
type ZoneAwareParams struct {
	// ZoneKey is the endpoint metadata key locality is matched on. The tag
	// supplies the shared default; stating it explicitly as empty is refused by
	// the expr, so "unset" and "deliberately blank" cannot be confused.
	ZoneKey string `value:"${zone-key:=zone}" expr:"$ != ''"`

	// Delegate names the sibling strategy the final choice among the local
	// subset is handed to. It is required, and it must not name this strategy
	// itself — both are relations to the enclosing strategy rather than
	// properties of the string, so [newZoneAware] owns them (an inner expr cannot
	// state "required only when the group is in play", and hardcoding the
	// strategy's own name here would duplicate the [ZoneAware] constant).
	//
	// The delegate's PARAMETERS are the selection's sibling groups
	// ([Selection.ConsistentHash], [Selection.Params]): zone_aware has no strategy
	// parameters of its own. A delegate that takes knobs therefore gets them from
	// the same rule, e.g. `consistent-hash.replicas` beside
	// `zone-aware.delegate=consistent_hash`.
	Delegate string `value:"${delegate:=}"`
}

// newZoneAware is the [Factory] behind the [ZoneAware] name. It composes with
// any sibling strategy, resolved by name out of the package registry.
func newZoneAware(sel Selection) (Balancer, error) {
	// An empty, unknown or self-naming delegate is a construction error, not a
	// silent fallback to round-robin: the filter only narrows the candidate set,
	// so the caller must say who makes the final choice.
	name := sel.ZoneAware.Delegate
	if name == "" {
		return nil, errutil.Explain(nil, "loadbalance: zone_aware requires a delegate")
	}
	if name == ZoneAware {
		return nil, errutil.Explain(nil, "loadbalance: zone_aware cannot delegate to itself")
	}
	// The delegate is built from THIS selection with the name swapped: the
	// strategy parameter groups describe the delegate, because zone_aware itself
	// has no strategy parameters. Without that, a delegate such as consistent_hash
	// would be built with zero knobs and fail on keyed traffic.
	bal, err := New(Selection{
		Balancer:       name,
		ConsistentHash: sel.ConsistentHash,
		Params:         sel.Params,
	})
	if err != nil {
		return nil, errutil.Explain(err, "loadbalance: zone_aware delegate %q", name)
	}
	return &zoneAware{zoneKey: sel.ZoneAware.ZoneKey, delegate: bal}, nil
}

// zoneAware is a locality-aware [Balancer], and a filter rather than a strategy:
// it narrows the candidate set to the best-matching zone level and delegates the
// actual choice, so it composes with any [Balancer] (round-robin inside the
// zone, least-conn, ...).
//
// It prefers endpoints whose Metadata[zoneKey] matches the caller's
// [PickInfo.Zone]. When the caller advertises no zone, or no endpoint matches any
// of its levels, it spills over to the full set so traffic is never black-holed
// just because the local zones are empty.
//
// PickInfo.Zone may carry a single zone ("cn-north-1a" — the classic usage) or
// an ordered fallback list ("cn-north-1a,cn-north-1": my rack, then my zone).
// Levels are tried left to right and the first level with at least one match
// wins; see [parseZoneLevels] for how a level matches an endpoint.
//
// This keeps traffic in-zone (lower latency, no cross-zone egress cost) under
// normal conditions while degrading gracefully during a zonal outage.
type zoneAware struct {
	zoneKey  string
	delegate Balancer
}

func (b *zoneAware) Pick(eps []discovery.Endpoint, info PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}
	levels := parseZoneLevels(info.Zone)
	if len(levels) == 0 {
		return b.delegate.Pick(eps, info)
	}

	for _, level := range levels {
		matched := make([]discovery.Endpoint, 0, len(eps))
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
func (b *zoneAware) Complete(ctx context.Context, ep discovery.Endpoint, err error) {
	b.delegate.Complete(ctx, ep, err)
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
