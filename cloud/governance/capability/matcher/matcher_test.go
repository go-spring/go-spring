/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * You may not use this file except in compliance with the License.
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

import "testing"

func TestTermMatches(t *testing.T) {
	tests := []struct {
		name string
		term Term
		v    string
		want bool
	}{
		{"exact hit", Term{Op: Exact, Value: "v2"}, "v2", true},
		{"exact miss", Term{Op: Exact, Value: "v2"}, "v20", false},
		{"exact empty matches empty", Term{Op: Exact, Value: ""}, "", true},
		{"prefix hit", Term{Op: Prefix, Value: "cn-north"}, "cn-north-1a", true},
		{"prefix miss", Term{Op: Prefix, Value: "cn-north"}, "cn-south-1a", false},
		{"suffix hit", Term{Op: Suffix, Value: "-1a"}, "cn-north-1a", true},
		{"suffix miss", Term{Op: Suffix, Value: "-1a"}, "cn-north-1b", false},
		{"contains hit", Term{Op: Contains, Value: "north"}, "cn-north-1a", true},
		{"contains miss", Term{Op: Contains, Value: "south"}, "cn-north-1a", false},
		// An unknown op is rejected at the boundary; if one reaches here it
		// must match nothing rather than everything.
		{"unknown op", Term{Op: Op("regex"), Value: ".*"}, "anything", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.term.Matches(tc.v); got != tc.want {
				t.Fatalf("%+v.Matches(%q) = %v, want %v", tc.term, tc.v, got, tc.want)
			}
		})
	}
}

func TestSelectorMatches(t *testing.T) {
	sel := Selector{
		{Key: "version", Op: Prefix, Value: "v2."},
		{Key: "zone", Op: Exact, Value: "cn-north-1a"},
	}
	tests := []struct {
		name string
		md   map[string]string
		want bool
	}{
		{"all terms satisfied", map[string]string{"version": "v2.1", "zone": "cn-north-1a"}, true},
		{"unnamed metadata is allowed", map[string]string{"version": "v2.1", "zone": "cn-north-1a", "tenant": "acme"}, true},
		{"one term fails", map[string]string{"version": "v2.1", "zone": "cn-south-1a"}, false},
		{"missing named key fails", map[string]string{"version": "v2.1"}, false},
		{"nil metadata fails", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sel.Matches(tc.md); got != tc.want {
				t.Fatalf("Matches(%v) = %v, want %v", tc.md, got, tc.want)
			}
		})
	}

	// Two terms may name the same key: they are a conjunction, so both must hold.
	sameKey := Selector{
		{Key: "version", Op: Prefix, Value: "v2."},
		{Key: "version", Op: Suffix, Value: "1"},
	}
	if !sameKey.Matches(map[string]string{"version": "v2.1"}) {
		t.Fatal("v2.1 should satisfy prefix v2. and suffix 1")
	}
	if sameKey.Matches(map[string]string{"version": "v2.10"}) {
		t.Fatal("v2.10 should fail suffix 1")
	}

	// The empty selector matches everything. That is the in-process "no
	// narrowing" default; a control-plane decoder must reject an explicitly
	// empty selector instead of installing it.
	if !(Selector(nil)).Matches(nil) || !(Selector{}).Matches(nil) {
		t.Fatal("empty selector should match everything")
	}

	// An empty operand still requires the key to be present.
	if (Selector{{Key: "tenant", Op: Exact, Value: ""}}).Matches(nil) {
		t.Fatal("exact-empty must not match a missing key")
	}
}

func TestSelectorValidate(t *testing.T) {
	valid := Selector{
		{Key: "version", Op: Prefix, Value: "v2."},
		{Key: "empty-value", Op: Exact, Value: ""},
		// A repeated key is a conjunction, not a conflict.
		{Key: "version", Op: Suffix, Value: "1"},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid selector rejected: %v", err)
	}

	// Key and Op are single-field rules owned by the field tags, so Validate
	// does not re-check them; the cases below are the cross-field rule it owns.
	tests := []struct {
		name string
		sel  Selector
	}{
		{"empty prefix", Selector{{Key: "v", Op: Prefix}}},
		{"empty suffix", Selector{{Key: "v", Op: Suffix}}},
		{"empty contains", Selector{{Key: "v", Op: Contains}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.sel.Validate(); err == nil {
				t.Fatalf("Validate accepted malformed selector %+v", tc.sel)
			}
		})
	}
}

func TestExactMapMatches(t *testing.T) {
	m := ExactMap{"tenant": "acme", "region": "cn"}
	tests := []struct {
		name string
		md   map[string]string
		want bool
	}{
		{"all pairs satisfied", map[string]string{"tenant": "acme", "region": "cn"}, true},
		{"unnamed metadata is allowed", map[string]string{"tenant": "acme", "region": "cn", "x": "y"}, true},
		{"wrong value", map[string]string{"tenant": "acme", "region": "us"}, false},
		{"missing key", map[string]string{"tenant": "acme"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.Matches(tc.md); got != tc.want {
				t.Fatalf("Matches(%v) = %v, want %v", tc.md, got, tc.want)
			}
		})
	}

	// The empty map matches everything, like the empty selector.
	if !(ExactMap(nil)).Matches(nil) {
		t.Fatal("empty map should match everything")
	}
	// An empty operand still requires the key to be present — the exact case of
	// Selector, so the two shapes cannot drift.
	if (ExactMap{"tenant": ""}).Matches(nil) {
		t.Fatal("empty value must not match a missing key")
	}
}

// TestExactMapEqualsExactSelector pins the invariant the two shapes share: an ExactMap means
// exactly the Selector built from the same pairs with Exact.
func TestExactMapEqualsExactSelector(t *testing.T) {
	m := ExactMap{"tenant": "acme", "region": "cn"}
	sel := Selector{
		{Key: "tenant", Op: Exact, Value: "acme"},
		{Key: "region", Op: Exact, Value: "cn"},
	}
	corpora := []map[string]string{
		nil,
		{},
		{"tenant": "acme"},
		{"tenant": "acme", "region": "cn"},
		{"tenant": "acme", "region": "cn", "x": "y"},
		{"tenant": "acme", "region": ""},
		{"tenant": "", "region": "cn"},
	}
	for _, md := range corpora {
		if m.Matches(md) != sel.Matches(md) {
			t.Fatalf("ExactMap and exact Selector disagree on %v", md)
		}
	}
}
