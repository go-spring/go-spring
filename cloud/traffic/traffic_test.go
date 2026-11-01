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
	"fmt"
	"net/http"
	"testing"

	"go-spring.org/cloud/propagate"
	"go-spring.org/stdlib/testing/assert"
)

// defaultMarker is go-spring's wire name, pinned independently of the
// implementation so a rename cannot silently split producers and consumers.
const defaultMarker = "x-traffic"

// mustProp returns the propagator over b.
func mustProp(t *testing.T, b Binding) Propagator {
	t.Helper()
	return NewDefaultPropagator(b)
}

// kindOf reports the kind p extracts when the carrier holds value under name.
func kindOf(p Propagator, name, value string) Kind {
	return p.Kind(p.Extract(context.Background(), marker(name, value)))
}

// marker is a carrier holding one value under a name, the shape every test hop
// takes.
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
// recognised by the next. The wire value is the kind itself, so the label and
// the marker share one spelling.
func TestDefaultBinding_CanonicalWireVocabulary(t *testing.T) {
	p := mustProp(t, DefaultBinding())

	assert.String(t, string(KindLoadTest)).Equal("loadtest")

	// The marker is written under one name, and read back under every casing a
	// protocol may give that name.
	c := written(p, p.WithKind(context.Background(), KindLoadTest))
	assert.That(t, len(c)).Equal(1)
	assert.That(t, c.Values(defaultMarker)).Equal([]string{"loadtest"})

	assert.That(t, kindOf(p, defaultMarker, "loadtest")).Equal(KindLoadTest)
	assert.That(t, kindOf(p, "X-Traffic", "loadtest")).Equal(KindLoadTest)
	// net/http canonicalises header keys on insert, so an HTTP header map holds
	// this spelling of the same name.
	assert.That(t, kindOf(p, "X-Traffic", "loadtest")).Equal(KindLoadTest)
	// A kind is a lower-case token: the wire spelling is folded, not trimmed.
	assert.That(t, kindOf(p, defaultMarker, "LoadTest")).Equal(KindLoadTest)
	assert.That(t, kindOf(p, defaultMarker, " loadtest ")).Equal(Kind(" loadtest "))
}

// ExampleDefaultPropagator shows the two seams an application actually uses:
// an inbound carrier tags the context, and an outbound carrier carries the same
// label on. Everything between them reads [Propagator.Kind].
func ExampleDefaultPropagator() {
	p := NewDefaultPropagator(DefaultBinding())

	// Inbound: a gateway stamped "loadtest" on the request header.
	in := propagate.Header(http.Header{"X-Traffic": {"loadtest"}})
	ctx := p.Extract(context.Background(), in)
	fmt.Println(IsLoadTest(ctx, p))

	// Outbound: the label rides on to the next hop.
	out := propagate.MultiMap{}
	p.Inject(ctx, out)
	fmt.Println(out.Values("x-traffic"))

	// Output:
	// true
	// [loadtest]
}

// TestKind_IsAnOpenVocabulary is the point of the type: the marker carries a
// label, not a bool, so a process with a classification of its own moves it
// across every hop verbatim and still asks the load-test question.
func TestKind_IsAnOpenVocabulary(t *testing.T) {
	p := mustProp(t, DefaultBinding())

	shadow := p.Extract(context.Background(), marker(defaultMarker, "Shadow"))
	assert.That(t, p.Kind(shadow)).Equal(Kind("shadow"))
	assert.That(t, IsLoadTest(shadow, p)).False()

	// The same label rides on: it is what the next hop reads.
	assert.That(t, written(p, shadow).Values(defaultMarker)).Equal([]string{"shadow"})
}

// TestKinds_CanonicalVocabulary pins the spellings go-spring ships. They are
// conventions, not a whitelist — the test exists so a rename cannot silently
// split producers and consumers — and each named kind must survive the full
// context → wire → context round trip.
func TestKinds_CanonicalVocabulary(t *testing.T) {
	p := mustProp(t, DefaultBinding())

	assert.String(t, string(KindLoadTest)).Equal("loadtest")
	assert.String(t, string(KindMirror)).Equal("mirror")
	assert.String(t, string(KindTest)).Equal("test")

	for _, kind := range []Kind{KindLoadTest, KindMirror, KindTest} {
		ctx := p.WithKind(context.Background(), kind)
		wire := written(p, ctx)
		assert.That(t, wire.Values(defaultMarker)).Equal([]string{string(kind)})
		assert.That(t, p.Kind(p.Extract(context.Background(), wire))).Equal(kind)
	}

	// Only one of the named kinds answers the load-test question.
	assert.That(t, IsLoadTest(p.WithKind(context.Background(), KindLoadTest), p)).True()
	for _, kind := range []Kind{KindMirror, KindTest} {
		assert.That(t, IsLoadTest(p.WithKind(context.Background(), kind), p)).False()
	}
}

// TestBinding_RebasedKeyKeepsTheRest is the re-basing idiom: a company renames
// the marker, leaves the rest of go-spring's convention in place.
func TestBinding_RebasedKeyKeepsTheRest(t *testing.T) {
	b := DefaultBinding()
	b.Key = "x-stress"
	p := mustProp(t, b)

	assert.That(t, kindOf(p, "x-stress", "loadtest")).Equal(KindLoadTest)
	assert.That(t, kindOf(p, "X-Stress", "loadtest")).Equal(KindLoadTest)
	// go-spring's own name is no longer this convention's.
	assert.That(t, kindOf(p, defaultMarker, "loadtest")).Equal(KindReal)

	c := written(p, p.WithKind(context.Background(), KindLoadTest))
	assert.That(t, len(c)).Equal(1)
	assert.That(t, c.Values("x-stress")).Equal([]string{"loadtest"})
}

// companyKey is the slot a company that already records its traffic
// classification on a context of its own supplies.
type companyKey struct{}

// TestBinding_OwnsTheSlot proves the context slot is the application's when it
// supplies one: another process's marker on the same context is not what it
// reads, so a re-based convention answers for its own key alone.
func TestBinding_OwnsTheSlot(t *testing.T) {
	b := DefaultBinding()
	b.WithKind = func(ctx context.Context, kind Kind) context.Context {
		return context.WithValue(ctx, companyKey{}, kind)
	}
	b.Kind = func(ctx context.Context) Kind {
		kind, _ := ctx.Value(companyKey{}).(Kind)
		return kind
	}
	p := mustProp(t, b)

	companyCtx := p.WithKind(context.Background(), KindLoadTest)
	assert.That(t, IsLoadTest(companyCtx, p)).True()

	// go-spring's own slot is invisible to this convention, and the company's is
	// invisible to go-spring's.
	goSpringCtx := mustProp(t, DefaultBinding()).WithKind(context.Background(), KindLoadTest)
	assert.That(t, p.Kind(goSpringCtx)).Equal(KindReal)
	assert.That(t, mustProp(t, DefaultBinding()).Kind(companyCtx)).Equal(KindReal)
}

// TestExtract_ReadsTheFirstNonEmptyValue pins what counts as the marker: an
// empty value is absent, and the first label under the key classifies the
// request. A later value cannot reclassify it, and neither can a carrier seen
// after the context already carries a kind.
func TestExtract_ReadsTheFirstNonEmptyValue(t *testing.T) {
	p := mustProp(t, DefaultBinding())
	mk := func(values ...string) Kind {
		return p.Kind(p.Extract(context.Background(), propagate.MultiMap{defaultMarker: values}))
	}

	assert.That(t, mk("", "loadtest")).Equal(KindLoadTest)
	assert.That(t, mk("loadtest", "shadow")).Equal(KindLoadTest)
	assert.That(t, mk("shadow", "loadtest")).Equal(Kind("shadow"))

	// An absent marker — no carrier, no key, an empty value — is real traffic,
	// not a panic.
	assert.That(t, mk()).Equal(KindReal)
	assert.That(t, mk("")).Equal(KindReal)
	assert.That(t, p.Kind(p.Extract(context.Background(), marker("X-Other", "loadtest")))).Equal(KindReal)
	assert.That(t, p.Kind(p.Extract(context.Background(), nil))).Equal(KindReal)

	// The label that classified the request first wins; a later hop does not
	// overwrite it.
	tagged := p.WithKind(context.Background(), Kind("shadow"))
	assert.That(t, p.Kind(p.Extract(tagged, marker(defaultMarker, "loadtest")))).Equal(Kind("shadow"))
}

// TestWithKind_IsIdempotent proves tagging an already-tagged context is a
// no-op rather than a second slot in the chain: every middleware on the path
// tags, so the cost and the shape must not grow with the chain length.
func TestWithKind_IsIdempotent(t *testing.T) {
	p := mustProp(t, DefaultBinding())

	once := p.WithKind(context.Background(), KindLoadTest)
	assert.That(t, p.WithKind(once, KindLoadTest)).Same(once)
	// The comparison happens after folding, so a differently-spelled argument
	// still hits the same branch.
	assert.That(t, p.WithKind(once, Kind("LOADTEST"))).Same(once)

	// Real traffic does not tag an unmarked context, and unmarked reads real.
	plain := context.Background()
	assert.That(t, p.WithKind(plain, KindReal)).Same(plain)
	assert.That(t, p.Kind(plain)).Equal(KindReal)

	// An explicit tag overwrites an earlier one; it is the caller saying so.
	assert.That(t, p.Kind(p.WithKind(once, Kind("shadow")))).Equal(Kind("shadow"))
}

// TestWithKind_SlotStoresUnconditionally pins where the checks live: the
// propagator folds the kind, drops [KindReal] and answers an already-tagged
// context itself, so the binding slot is reached only when it must store.
func TestWithKind_SlotStoresUnconditionally(t *testing.T) {
	b := DefaultBinding()
	slot := DefaultBinding()
	var stored []Kind
	b.WithKind = func(ctx context.Context, kind Kind) context.Context {
		stored = append(stored, kind)
		return slot.WithKind(ctx, kind)
	}
	p := mustProp(t, b)
	plain := context.Background()

	// Real traffic is answered before the slot.
	assert.That(t, p.WithKind(plain, KindReal)).Same(plain)
	assert.That(t, len(stored)).Equal(0)

	// The first tag reaches the slot, already folded.
	tagged := p.WithKind(plain, Kind("LoadTest"))
	assert.That(t, stored).Equal([]Kind{KindLoadTest})

	// The same kind, spelled differently, is answered before the slot.
	assert.That(t, p.WithKind(tagged, Kind("LOADTEST"))).Same(tagged)
	assert.That(t, len(stored)).Equal(1)
}

// legacyPropagator is the documented escape hatch for a wire spelling that
// differs from the kind: a legacy gateway writes "pts" for what this process
// calls loadtest. Only the two readers are overridden; the embedded seams keep
// moving the literal label, so Inject still writes "pts" on the wire.
type legacyPropagator struct{ Propagator }

func (p legacyPropagator) Kind(ctx context.Context) Kind {
	if kind := p.Propagator.Kind(ctx); kind == "pts" {
		return KindLoadTest
	}
	return p.Propagator.Kind(ctx)
}

func (p legacyPropagator) WithKind(ctx context.Context, kind Kind) context.Context {
	if kind == KindLoadTest {
		kind = "pts"
	}
	return p.Propagator.WithKind(ctx, kind)
}

// TestCustomPropagatorRemapsLabels proves a label mapping belongs to a custom
// [Propagator], not to [Binding]: "pts" reads as load-test traffic here and is
// written back as "pts" for the next legacy hop.
func TestCustomPropagatorRemapsLabels(t *testing.T) {
	p := legacyPropagator{mustProp(t, DefaultBinding())}

	ctx := p.Extract(context.Background(), marker(defaultMarker, "pts"))
	assert.That(t, p.Kind(ctx)).Equal(KindLoadTest)
	assert.That(t, IsLoadTest(ctx, p)).True()

	out := propagate.MultiMap{}
	p.Inject(ctx, out)
	assert.That(t, out.Values(defaultMarker)).Equal([]string{"pts"})
}

// TestPropagate_CarriesTheKindAcrossABoundary covers the goroutine hop: work
// fanned out on a fresh context keeps the parent's kind, and real traffic does
// not tag the child.
func TestPropagate_CarriesTheKindAcrossABoundary(t *testing.T) {
	p := mustProp(t, DefaultBinding())

	loadParent := p.WithKind(context.Background(), KindLoadTest)
	assert.That(t, p.Kind(Propagate(loadParent, p, context.Background()))).Equal(KindLoadTest)

	shadowParent := p.WithKind(context.Background(), Kind("shadow"))
	assert.That(t, p.Kind(Propagate(shadowParent, p, context.Background()))).Equal(Kind("shadow"))

	// A child already carrying another kind takes the parent's: the parent is
	// the explicit statement of what the call is.
	child := p.WithKind(context.Background(), Kind("shadow"))
	assert.That(t, p.Kind(Propagate(loadParent, p, child))).Equal(KindLoadTest)

	// Real traffic is not tagged by the hop, and the child is returned as-is.
	plain := context.Background()
	assert.That(t, Propagate(plain, p, plain)).Same(plain)
}

// TestInject_InertForRealTraffic proves the seam only injects for classified
// traffic: a synthetic marker on a production request would be worse than
// carrying none.
func TestInject_InertForRealTraffic(t *testing.T) {
	p := mustProp(t, DefaultBinding())
	ctx := context.Background()

	assert.That(t, len(written(p, ctx))).Equal(0)

	// A nil carrier drops the write rather than panicking.
	p.Inject(p.WithKind(ctx, KindLoadTest), nil)
	assert.That(t, p.Extract(ctx, nil)).Same(ctx)
}
