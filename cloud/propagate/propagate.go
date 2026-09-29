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

// Package propagate carries markers across protocol boundaries.
//
// [Carrier] is the three-method seam every protocol's metadata adapts to —
// enumerate keys, read a key's values, write a key — so a marker moves across
// an HTTP header, a gRPC metadata.MD, a Kafka record's header slice or a
// dubbo attachment map through one shape. Three adapters cover the common map
// forms ([MultiMap], [StringMap], [Header]) as zero-copy views over the
// protocol's own map; anything else (a header slice, a getter/setter pair)
// implements the interface in a small adapter type where it lives — the
// decode rule that is a protocol fact, not carrier knowledge, stays with the
// protocol.
//
// The carrier is half the pattern; the marker's owner holds the other half.
// One marker is one key plus a seam pair:
//
//	// inbound, per server hop: carrier → ctx
//	for _, v := range c.Values(key) {
//	    if accepts(v) {
//	        ctx = withMarker(ctx, v)
//	    }
//	}
//	// outbound, per client hop: ctx → carrier
//	if v, ok := fromMarker(ctx); ok {
//	    c.Set(key, v)
//	}
//
// cloud/governance/traffic is the reference pair for the load-test marker.
// This package holds no policy — no key registry, no value rules, no context
// slot — only the carrier's invariants: an empty value reads as absent, and
// Set replaces instead of appending (so re-injecting is idempotent). A view
// over a nil map reads empty; writing needs a non-nil map, so an outbound
// seam that injects allocates its map first.
//
// One boundary. For trace/correlation propagation prefer the OpenTelemetry
// propagator API — this package is for domain markers that ride a self-owned
// contract.
package propagate

import (
	"maps"
	"net/http"
	"slices"
)

// Carrier is the protocol-neutral metadata seam: enumerate the keys the hop's
// metadata holds, read one key's values, write one key. Every adapter reads a
// nil-or-absent key as no values, and Set replaces rather than appends.
type Carrier interface {
	// Keys returns every key the carrier holds, in no particular order.
	Keys() []string
	// Values returns the values stored under key, nil when the key is absent
	// or carries only empty values.
	Values(key string) []string
	// Set stores value under key, replacing any previous values rather than
	// appending — re-injecting a marker is idempotent, never a second entry.
	Set(key, value string)
}

// MultiMap is a Carrier over a string multi-map: a gRPC metadata.MD, a raw
// multi-value header map. A zero-copy view — Set writes through to m — whose
// keys are matched exactly as stored; see [Header] for the map whose keys
// follow net/http's canonical spelling.
type MultiMap map[string][]string

// Keys returns the map's keys, in no particular order.
func (m MultiMap) Keys() []string { return slices.Collect(maps.Keys(m)) }

// Values returns the raw values stored under key, nil when the key is absent.
func (m MultiMap) Values(key string) []string {
	if vs := m[key]; len(vs) > 0 {
		return vs
	}
	return nil
}

// Set stores value under key, replacing.
func (m MultiMap) Set(key, value string) {
	m[key] = []string{value}
}

// StringMap is a Carrier over a single-value map — a Pulsar or RocketMQ
// property set. A view like [MultiMap].
type StringMap map[string]string

// Keys returns the map's keys, in no particular order.
func (s StringMap) Keys() []string { return slices.Collect(maps.Keys(s)) }

// Values returns the value stored under key, nil when it is absent or empty —
// reading an empty value as absent is what aligns the carrier with an absent
// header, so one acceptance rule serves both.
func (s StringMap) Values(key string) []string {
	if v := s[key]; v != "" {
		return []string{v}
	}
	return nil
}

// Set stores value under key, replacing.
func (s StringMap) Set(key, value string) {
	s[key] = value
}

// Header is a Carrier over an [net/http.Header]: Values and Set go through
// Header.Values and Header.Set, so lookups are case-insensitive and writes
// land in canonical spelling — the write rule that keeps [MultiMap] from
// serving an http.Header itself. A view like [MultiMap].
type Header http.Header

// Keys returns the header's keys as stored — textproto-canonicalised — in no
// particular order. Match them case-insensitively.
func (h Header) Keys() []string { return slices.Collect(maps.Keys(h)) }

// Values returns the values under key, matched case-insensitively the way
// [net/http.Header.Values] matches.
func (h Header) Values(key string) []string {
	return http.Header(h).Values(key)
}

// Set stores value under key in canonical spelling, replacing.
func (h Header) Set(key, value string) {
	http.Header(h).Set(key, value)
}
