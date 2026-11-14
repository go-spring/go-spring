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

package chain

import "context"

// Options is the per-call input a CALLER states for the outbound runtime —
// the one handle a caller has, since callers never see the [Operation] a
// starter declares on their behalf. It is the caller-side channel of the
// chain: whatever a call may legitimately ask of the runtime that is not a
// governance setting rides here, as a named field with its consumer in the
// runtime. It is deliberately a struct, not a bag: a new capability joins by
// adding a field here plus its consumer — never by widening this into a
// generic "chain parameters" map, which would open a second configuration
// channel on the data plane.
//
// A field left at its zero value means "no input on this dimension"; the zero
// [Options] as a whole changes nothing about how the call runs.
type Options struct {
	// Zone is the downstream zone the calls made under it should target —
	// treated as the downstream CLUSTER for now. It feeds the balancer's zone
	// input per round (retry re-picks included), so it applies to every round
	// of the call.
	Zone string

	// HashKey is the caller-stated affinity key: requests carrying the same
	// key map to the same endpoint under a keyed (consistent-hash) strategy.
	// It states the RAW key — the balancer hashes it itself — and whether to
	// key at all, and on what dimension, is the caller's decision: it wins
	// over any key the starter derived from the call (the redis command key).
	// It is read per round, so retry re-picks stay affine.
	HashKey string
}

// optionsKey is the context key the caller's per-call options ride under: an
// unexported struct type, the usual context-key discipline so no other package
// can collide with it, and its value is an [Options].
type optionsKey struct{}

// WithOptions returns a copy of ctx carrying o as the caller's per-call
// options. A later call replaces an earlier one — the last binding wins.
func WithOptions(ctx context.Context, o Options) context.Context {
	return context.WithValue(ctx, optionsKey{}, o)
}

// OptionsFrom returns the options ctx carries and whether the caller bound
// any. The bool is EXISTENCE of the slot, not a type check in practice: the
// key is unexported and [WithOptions] is its only writer, so a value of
// another type can never sit under it — the type assertion's ok and "the
// caller bound options" are the same predicate by construction. It is the
// read half of the [WithOptions] slot; the runtime's outbound faces (today
// the loadbalance picker) are its consumers.
func OptionsFrom(ctx context.Context) (Options, bool) {
	o, ok := ctx.Value(optionsKey{}).(Options)
	return o, ok
}
