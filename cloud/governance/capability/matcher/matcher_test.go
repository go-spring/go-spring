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

package matcher

import (
	"strings"
	"testing"
)

func TestStringMatchValidate(t *testing.T) {
	valid := []StringMatch{
		{Exact: "acme"},
		{Prefix: "v2."},
		{Suffix: "-1a"},
		{Contains: "north"},
	}
	for i, m := range valid {
		if err := m.Validate(); err != nil {
			t.Fatalf("valid match %d rejected: %v", i, err)
		}
	}
	invalid := []struct {
		name string
		m    StringMatch
		want string
	}{
		// An empty operand does not bind, so an all-empty value is "no op
		// written" and a present-but-empty value cannot be targeted.
		{"no op", StringMatch{}, "sets no op"},
		{"two ops", StringMatch{Exact: "a", Prefix: "b"}, "more than one op"},
		{"three ops", StringMatch{Prefix: "a", Suffix: "b", Contains: "c"}, "more than one op"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.m.Validate()
			if err == nil {
				t.Fatalf("Validate accepted malformed match %+v", tc.m)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

// TestActionMatchMatches pins the atomic predicate, including the service
// marker: "-" means "the whole service", so it matches every action.
func TestActionMatchMatches(t *testing.T) {
	tests := []struct {
		name string
		m    ActionMatch
		act  string
		want bool
	}{
		{"zero matches all", ActionMatch{}, "GET /x", true},
		{"service marker matches all", ActionMatch{Exact: ServiceScopeAction}, "GET /x", true},
		{"exact matches itself", ActionMatch{Exact: "insert"}, "insert", true},
		{"exact does not match another", ActionMatch{Exact: "insert"}, "update", false},
		{"prefix matches subtree", ActionMatch{Prefix: "GET /users"}, "GET /users/1", true},
		{"prefix does not match another verb", ActionMatch{Prefix: "GET /users"}, "POST /users", false},
		{"exact wins over prefix", ActionMatch{Exact: "insert", Prefix: "in"}, "insert", true},
	}
	for _, tc := range tests {
		if got := tc.m.Matches(tc.act); got != tc.want {
			t.Fatalf("%s: %+v.Matches(%q) = %v, want %v", tc.name, tc.m, tc.act, got, tc.want)
		}
	}
}

// TestActionMatchBetterThan pins the ranking: exact > longer prefix >
// shorter prefix > service scope, and equal selectors do not outrank each other.
func TestActionMatchBetterThan(t *testing.T) {
	exact := ActionMatch{Exact: "GET /users/1"}
	longer := ActionMatch{Prefix: "GET /users"}
	shorter := ActionMatch{Prefix: "GET /"}
	service := ActionMatch{Exact: ServiceScopeAction}
	zero := ActionMatch{}

	if !exact.BetterThan(longer) {
		t.Fatal("exact must beat a prefix")
	}
	if !longer.BetterThan(shorter) {
		t.Fatal("a longer prefix must beat a shorter one")
	}
	if !shorter.BetterThan(service) {
		t.Fatal("a real prefix must beat the service scope")
	}
	if !longer.BetterThan(zero) {
		t.Fatal("a non-empty prefix must beat the zero selector")
	}
	if shorter.BetterThan(shorter) {
		t.Fatal("equal selectors must not outrank each other")
	}
}

func TestStringMatchMatches(t *testing.T) {
	tests := []struct {
		name  string
		m     StringMatch
		v     string
		match bool
	}{
		{"exact hit", StringMatch{Exact: "v2"}, "v2", true},
		{"exact miss", StringMatch{Exact: "v2"}, "v20", false},
		{"prefix hit", StringMatch{Prefix: "cn-north"}, "cn-north-1a", true},
		{"prefix miss", StringMatch{Prefix: "cn-north"}, "cn-south-1a", false},
		{"suffix hit", StringMatch{Suffix: "-1a"}, "cn-north-1a", true},
		{"suffix miss", StringMatch{Suffix: "-1a"}, "cn-north-1b", false},
		{"contains hit", StringMatch{Contains: "north"}, "cn-north-1a", true},
		{"contains miss", StringMatch{Contains: "south"}, "cn-north-1a", false},
		// The operand may cover the whole value: prefix/suffix/contains are
		// satisfied by an equal value, so "exact" is the spelling for it, not
		// a different outcome.
		{"prefix is the whole value", StringMatch{Prefix: "v2"}, "v2", true},
		{"suffix is the whole value", StringMatch{Suffix: "v2"}, "v2", true},
		{"contains is the whole value", StringMatch{Contains: "v2"}, "v2", true},
		// A value that bypassed Validate must match nothing rather than
		// everything.
		{"no op fails closed", StringMatch{}, "anything", false},
		// Validation rejects two set ops, but a programmatically built value
		// can still carry them: pin the precedence the evaluation switch
		// gives, so the behavior is a convention rather than an accident.
		{"exact wins over prefix", StringMatch{Exact: "a", Prefix: "a"}, "a", true},
		{"exact wins over prefix, miss", StringMatch{Exact: "a", Prefix: "ab"}, "ab", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.Matches(tc.v); got != tc.match {
				t.Fatalf("%+v.Matches(%q) = %v, want %v", tc.m, tc.v, got, tc.match)
			}
		})
	}
}

// TestMatches pins the one evaluation path: subset-match, presence required,
// extra metadata allowed, entries ANDed. Repeating a key is impossible by
// construction — a map holds one condition per key.
func TestMetadataMatches(t *testing.T) {
	match := map[string]StringMatch{
		"version": {Prefix: "v2."},
		"zone":    {Exact: "cn-north-1a"},
	}
	tests := []struct {
		name string
		md   map[string]string
		want bool
	}{
		{"all entries satisfied", map[string]string{"version": "v2.1", "zone": "cn-north-1a"}, true},
		{"unnamed metadata is allowed", map[string]string{"version": "v2.1", "zone": "cn-north-1a", "tenant": "acme"}, true},
		{"one entry fails", map[string]string{"version": "v2.1", "zone": "cn-south-1a"}, false},
		{"missing named key fails", map[string]string{"version": "v2.1"}, false},
		// Presence is not enough: a present-but-empty value cannot be
		// targeted, because an empty operand does not bind (see [StringMatch]).
		{"present-but-empty value fails", map[string]string{"version": "v2.1", "zone": ""}, false},
		{"nil metadata fails", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MetadataMatches(match, tc.md); got != tc.want {
				t.Fatalf("MetadataMatches(%v) = %v, want %v", tc.md, got, tc.want)
			}
		})
	}

	// The empty map matches everything. That is the in-process "no narrowing"
	// default; a control-plane decoder must reject an explicitly empty map
	// instead of installing it.
	if !MetadataMatches(nil, nil) || !MetadataMatches(map[string]StringMatch{}, nil) {
		t.Fatal("empty map should match everything")
	}
}
