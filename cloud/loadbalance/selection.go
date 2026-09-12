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
	"sync/atomic"
	"time"
)

// Selection is a resolved endpoint-selection policy in plain values: the
// strategy name and the outlier-suspension thresholds. It mirrors
// [Pool.ApplySelection] so this package carries no dependency on the policy
// model that produces it.
type Selection struct {
	// Balancer is the strategy name to select endpoints with (e.g.
	// "least_conn"). Empty means "leave the pool's current strategy alone".
	Balancer string

	// OutlierThreshold is the consecutive-failure count that suspends an
	// endpoint. 0 disables suspension.
	OutlierThreshold int

	// OutlierSuspendFor is the cool-down before a suspended endpoint gets a
	// half-open trial request. Ignored when [Selection.OutlierThreshold] is 0.
	OutlierSuspendFor time.Duration
}

// SelectionProvider resolves the current [Selection] for a resource label and
// keeps the caller in step with it. The contract is:
//
//   - invoke apply immediately with the current Selection, so a pool is already
//     under its policy before the first Pick rather than only after the next
//     push;
//   - invoke apply again on every change;
//   - return an idempotent stop func that detaches the subscription (nil means
//     "nothing to stop").
//
// It is the selection counterpart of [resilience.Provider]: the governance
// authority registers one process-wide, and clients reach it through
// [Pool.BindSelection] without naming the authority or its policy model.
type SelectionProvider func(label string, apply func(Selection)) (stop func())

// selectionProvider is the process-wide provider, installed by the governance
// authority once it is live. nil (the zero value) means endpoint selection is
// unmanaged for the whole process — [Pool.BindSelection] is then a no-op and
// each pool keeps the strategy it was built with.
var selectionProvider atomic.Pointer[SelectionProvider]

// RegisterSelectionProvider installs p as the process-wide selection provider.
// The governance authority calls this once, when it goes live; clients never
// call it. Passing nil DISARMS the seam — every later [Pool.BindSelection]
// returns a no-op stop, and pools already bound keep living under the last
// policy they were given. Disarming is what lets in-process tests install a
// fake provider and take it back out again.
func RegisterSelectionProvider(p SelectionProvider) {
	if p == nil {
		selectionProvider.Store(nil)
		return
	}
	selectionProvider.Store(&p)
}

// BindSelection wires the pool's endpoint selection to label's managed policy:
// the current policy is applied immediately and again on every change, in
// place — no pool rebuild, and the next [Pool.Pick] already sees it.
//
// It returns the detach func. A pool whose lifetime is not the whole process
// MUST call it — exactly as a caller must cancel a governance subscription;
// otherwise the authority keeps a callback pointing at a dead pool. With no
// provider registered it returns a no-op and costs a single atomic load, which
// is the transparent pass-through in a process without the governance starter.
//
// The strategy half works on any pool. The suspension half needs a [Tracker]
// attached (see [WithTracker]) and the caller to pair every [Pool.Pick] with a
// [Pool.Complete] — without both, the thresholds are set on nothing.
func (p *Pool) BindSelection(label string) (stop func()) {
	prov := selectionProvider.Load()
	if prov == nil || *prov == nil {
		return func() {}
	}
	stop = (*prov)(label, func(s Selection) {
		p.ApplySelection(s.Balancer, s.OutlierThreshold, s.OutlierSuspendFor)
	})
	if stop == nil {
		return func() {}
	}
	return stop
}

// Selection returns the endpoint-selection policy most recently applied to the
// pool through [Pool.ApplySelection], or the zero Selection when none ever was.
//
// The strategy name is the last one *accepted*: applying an empty name leaves
// it untouched, and an unknown name is ignored, so the name keeps describing
// the strategy actually in force. [Pool.SetBalancer] called directly does not
// update it — it takes an instance, not a name. It is an inspection and test
// helper.
func (p *Pool) Selection() Selection {
	if s := p.sel.Load(); s != nil {
		return *s
	}
	return Selection{}
}
