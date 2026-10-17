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

package StarterGateway

import (
	"context"
	"strings"
	"testing"

	"go-spring.org/cloud/runtime/dataplane/loadbalance"
)

// newTestTable builds a RouteTable without the container, enough for
// recompile: direct http upstreams need no discovery or wrappers.
func newTestTable(t *testing.T) *RouteTable {
	t.Helper()
	lbMgr := loadbalance.NewManager()
	tbl := &RouteTable{
		ctx:     context.Background(),
		obs:     newObserver(),
		ctr:     nil, // no governance: the center's methods are nil-safe, giving observed pass-throughs
		lbMgr:   lbMgr,
		limiter: newMemoryLimiter(),
	}
	return tbl
}

func TestRecompilePriorityOrder(t *testing.T) {
	tbl := newTestTable(t)
	raw := map[string]RouteRaw{
		"a-first": {Upstream: struct { // id sorts first, no priority
			Target    string `value:"${target:=}"`
			Discovery string `value:"${discovery:=}"`
		}{Target: "http://127.0.0.1:19000"}},
		"b-catchall": {Priority: 10, Upstream: struct {
			Target    string `value:"${target:=}"`
			Discovery string `value:"${discovery:=}"`
		}{Target: "http://127.0.0.1:19000"}},
		"c-second": {Priority: 5, Upstream: struct {
			Target    string `value:"${target:=}"`
			Discovery string `value:"${discovery:=}"`
		}{Target: "http://127.0.0.1:19000"}},
		"d-tie": {Priority: 10, Upstream: struct { // ties with b-catchall -> id order
			Target    string `value:"${target:=}"`
			Discovery string `value:"${discovery:=}"`
		}{Target: "http://127.0.0.1:19000"}},
	}
	if err := tbl.recompile(raw); err != nil {
		t.Fatalf("recompile: %v", err)
	}
	routes := *tbl.compiled.Load()
	got := make([]string, len(routes))
	for i, r := range routes {
		got[i] = r.ID
	}
	want := []string{"b-catchall", "d-tie", "c-second", "a-first"} // priority desc, id asc within ties
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("route order = %v, want %v", got, want)
	}
}

func TestRecompileDefaultOrderUnchanged(t *testing.T) {
	tbl := newTestTable(t)
	raw := map[string]RouteRaw{}
	for _, id := range []string{"delta", "alpha", "charlie"} {
		r := RouteRaw{}
		r.Upstream.Target = "http://127.0.0.1:19000"
		raw[id] = r
	}
	if err := tbl.recompile(raw); err != nil {
		t.Fatalf("recompile: %v", err)
	}
	routes := *tbl.compiled.Load()
	var got []string
	for _, r := range routes {
		got = append(got, r.ID)
	}
	want := "alpha,charlie,delta"
	if strings.Join(got, ",") != want {
		t.Fatalf("route order = %v, want %s", got, want)
	}
}

func TestParseFilterTokenRejectsReservedChars(t *testing.T) {
	for _, spec := range []string{
		"addRequestHeader(X-Foo,a(b))",
		"addRequestHeader(X-Foo,a)b",
		"prefixPath(a(b)",
	} {
		if _, err := splitFilters(spec); err == nil {
			t.Fatalf("splitFilters(%q): expected error", spec)
		}
	}
	// A paren inside a balanced token reaches the value check and names the
	// alternatives explicitly.
	if _, err := splitFilters("addRequestHeader(X-Foo,a(b))"); err == nil ||
		!strings.Contains(err.Error(), "no escape") {
		t.Fatalf("expected 'no escape' guidance, got %v", err)
	}
}

func TestParseFilterTokenAcceptsPlainValues(t *testing.T) {
	toks, err := splitFilters("stripPrefix(1),addRequestHeader(X-From,gw),requestId()")
	if err != nil {
		t.Fatalf("splitFilters: %v", err)
	}
	if len(toks) != 3 || toks[0].name != "stripPrefix" || toks[1].args[1] != "gw" || len(toks[2].args) != 1 {
		t.Fatalf("unexpected tokens: %+v", toks)
	}
}

// The per-route governance registration used to be reconciled (bound and
// detached) on every recompile. It is now established ONCE, when the chain is
// built, and the chain itself is cached per route id across recompiles —
// registration is one-way (see loadbalance.Manager.Chain) means a fresh chain per
// edit would strand the old registration and pin a discarded chain for the life
// of the process. The cache
// is exercised through [RouteTable.chainFor], which needs a live discovery
// backend, so the invariant is covered by the example and the loadbalance
// package rather than here.
