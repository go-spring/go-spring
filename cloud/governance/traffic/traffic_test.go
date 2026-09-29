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

package traffic

import (
	"context"
	"testing"

	"go-spring.org/cloud/propagate"
	"go-spring.org/stdlib/testing/assert"
)

// mustProp returns the propagator over b, failing the test on an error the
// constructor never actually returns (the signature keeps the room).
func mustProp(t *testing.T, b Binding) Propagator {
	t.Helper()
	p, err := NewDefaultPropagator(b)
	assert.Error(t, err).Nil()
	return p
}

// tagged reports whether p tags a plain context when the carrier holds value
// under the marker's name.
func tagged(p Propagator, c propagate.Carrier) bool {
	return p.IsLoadTest(p.Extract(context.Background(), c))
}

// marker is a carrier holding one value under a name, the shape every test
// hop takes.
func marker(name, value string) propagate.Carrier {
	return propagate.MultiMap{name: {value}}
}

// written returns the entries p writes onto a fresh multi-map.
func written(p Propagator, ctx context.Context) propagate.MultiMap {
	c := propagate.MultiMap{}
	p.Inject(ctx, c)
	return c
}

// TestDefaultBinding_CanonicalWireVocabulary pins what a process gets when it
// re-bases nothing: without it, a marker written by one hop would not be
// recognised by the next.
func TestDefaultBinding_CanonicalWireVocabulary(t *testing.T) {
	p := mustProp(t, DefaultBinding())

	assert.String(t, canonicalKey).Equal("x-loadtest")
	assert.String(t, canonicalValue).Equal("1")

	// The marker is written under one name, and read back under every casing a
	// protocol may give that name.
	c := written(p, p.WithLoadTest(context.Background()))
	assert.That(t, len(c)).Equal(1)
	assert.That(t, c.Values(canonicalKey)).Equal([]string{canonicalValue})

	assert.That(t, tagged(p, marker(canonicalKey, canonicalValue))).True()
	assert.That(t, tagged(p, marker("X-LoadTest", canonicalValue))).True()
	// net/http canonicalises header keys on insert, so an HTTP header map holds
	// this spelling of the same name.
	assert.That(t, tagged(p, marker("X-Loadtest", canonicalValue))).True()
}

// TestBinding_RebasedKeyKeepsTheRest is the re-basing idiom: a company renames
// the marker, leaves the rest of go-spring's convention in place.
func TestBinding_RebasedKeyKeepsTheRest(t *testing.T) {
	b := DefaultBinding()
	b.Key = "x-stress"
	p := mustProp(t, b)

	assert.That(t, tagged(p, marker("x-stress", "1"))).True()
	assert.That(t, tagged(p, marker("X-Stress", "1"))).True()
	// go-spring's own name is no longer this convention's.
	assert.That(t, tagged(p, marker(canonicalKey, "1"))).False()

	c := written(p, p.WithLoadTest(context.Background()))
	assert.That(t, len(c)).Equal(1)
	assert.That(t, c.Values("x-stress")).Equal([]string{"1"})
}

// companyKey is the slot a company that already records synthetic traffic on a
// context of its own supplies.
type companyKey struct{}

// TestBinding_OwnsTheSlot proves the context slot is the application's when it
// supplies one: another process's marker on the same context is not what it
// reads, so a re-based convention answers for its own key alone.
func TestBinding_OwnsTheSlot(t *testing.T) {
	b := DefaultBinding()
	b.Bind = func(ctx context.Context) context.Context {
		if ctx == nil {
			ctx = context.Background()
		}
		return context.WithValue(ctx, companyKey{}, true)
	}
	b.Bound = func(ctx context.Context) bool {
		return ctx != nil && ctx.Value(companyKey{}) != nil
	}
	p := mustProp(t, b)

	companyCtx := p.WithLoadTest(context.Background())
	assert.That(t, p.IsLoadTest(companyCtx)).True()

	// go-spring's own slot is invisible to this convention, and the company's is
	// invisible to go-spring's.
	goSpringCtx := mustProp(t, DefaultBinding()).WithLoadTest(context.Background())
	assert.That(t, p.IsLoadTest(goSpringCtx)).False()
	assert.That(t, mustProp(t, DefaultBinding()).IsLoadTest(companyCtx)).False()
}

// TestBinding_RebasedValue proves the value can be re-based: a company
// whose gateway stamps "synthetic" must read its own spelling — and only it.
func TestBinding_RebasedValue(t *testing.T) {
	b := DefaultBinding()
	b.Value = "synthetic"
	p := mustProp(t, b)

	assert.That(t, tagged(p, marker(canonicalKey, "synthetic"))).True()
	assert.That(t, tagged(p, marker(canonicalKey, "SYNTHETIC"))).True()
	// go-spring's own spelling no longer asserts the marker.
	assert.That(t, tagged(p, marker(canonicalKey, "1"))).False()
}

// TestExtract_AcceptsOnlyTheValue pins what counts as the marker: a hop
// that forwards any other value must not turn real traffic into load-test
// traffic.
func TestExtract_AcceptsOnlyTheValue(t *testing.T) {
	p := mustProp(t, DefaultBinding())

	assert.That(t, tagged(p, marker(canonicalKey, "1"))).True()
	assert.That(t, tagged(p, marker(canonicalKey, "TRUE"))).False()
	for _, v := range []string{"", "0", "true", "on", "yes", "2"} {
		assert.That(t, tagged(p, marker(canonicalKey, v))).False()
	}

	// Other keys carry real-traffic headers, never the marker.
	assert.That(t, tagged(p, marker("X-Other", "1"))).False()
}

// TestExtract_ReadsEveryValue proves a multi-valued key (gRPC metadata) tags
// the context when ANY value under it is the wire value, and leaves a carrier
// holding only real-traffic values alone.
func TestExtract_ReadsEveryValue(t *testing.T) {
	p := mustProp(t, DefaultBinding())
	mk := func(values ...string) context.Context {
		return p.Extract(context.Background(), propagate.MultiMap{canonicalKey: values})
	}

	assert.That(t, p.IsLoadTest(mk("0", "1"))).True()
	assert.That(t, p.IsLoadTest(mk("0", "0"))).False()
	assert.That(t, p.IsLoadTest(mk())).False()

	// An absent carrier is plain traffic, not a panic.
	assert.That(t, p.IsLoadTest(p.Extract(context.Background(), nil))).False()
}

// TestWithLoadTest_IsIdempotent proves tagging an already-tagged context is a
// no-op rather than a second slot in the chain: every middleware on the path
// tags, so the cost and the shape must not grow with the chain length.
func TestWithLoadTest_IsIdempotent(t *testing.T) {
	p := mustProp(t, DefaultBinding())

	once := p.WithLoadTest(context.Background())
	assert.That(t, p.IsLoadTest(p.WithLoadTest(once))).True()

	// A nil context is treated as the background, never a panic.
	assert.That(t, p.IsLoadTest(p.WithLoadTest(nil))).True()
	assert.That(t, p.IsLoadTest(nil)).False()
}

// TestPropagate_CarriesTheFlagAcrossABoundary covers the goroutine hop: work
// fanned out on a fresh context stays synthetic only when the parent was.
func TestPropagate_CarriesTheFlagAcrossABoundary(t *testing.T) {
	p := mustProp(t, DefaultBinding())

	loadParent := p.WithLoadTest(context.Background())
	assert.That(t, p.IsLoadTest(Propagate(p, loadParent, context.Background()))).True()

	// Real traffic is not tagged by the hop, and the child is returned as-is.
	plain := context.Background()
	assert.That(t, Propagate(p, plain, plain)).Same(plain)
}

// TestInject_InertForRealTraffic proves the seam only injects for load-test
// traffic: a synthetic marker on a production request would be worse than
// carrying none.
func TestInject_InertForRealTraffic(t *testing.T) {
	p := mustProp(t, DefaultBinding())
	ctx := context.Background()

	assert.That(t, len(written(p, ctx))).Equal(0)

	// A nil carrier drops the write rather than panicking.
	p.Inject(p.WithLoadTest(ctx), nil)
	assert.That(t, p.Extract(ctx, nil)).Same(ctx)
}
