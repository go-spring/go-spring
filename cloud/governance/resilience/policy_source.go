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

// policy_source.go is the PULL seam between the manager and the executors it
// built: what an executor asks its manager on every call, and how the manager
// answers without taking a lock or touching its registry.
//
// It replaces the push side of the old protocol. A policy change used to arrive
// at the executor through [Refreshable]/[ServerEngine.Refresh], which made the
// manager walk its registrations and retune every executor in place. Now a push
// stops at the manager: [ClientManager.Apply] publishes a new snapshot, and each
// executor notices on its next call that the GENERATION it was built from is
// gone and rebuilds its state from scratch.
//
// The generation is what makes that safe to do lazily. It is derived from the
// CONTENT of the rule set a label resolves against, never from a push counter:
// a document re-pushed byte-identical (a file watcher firing twice, a poll that
// changed nothing) must not reset a live breaker, and the generation is how the
// executor tells "the config moved" from "someone handed me the same config
// again".

package resilience

import (
	"fmt"
	"hash"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"

	"go-spring.org/cloud/chain"
)

// CallPolicy is one outbound call's protection as its manager resolves it: the
// policy the call runs under, the key its state is partitioned by, the
// label-level policy the executor-wide organs are sized from, and the generation
// all three came from.
type CallPolicy struct {
	// Policy is what this call runs under: the matched rule's policy, or the
	// default when no rule matches.
	Policy ClientPolicy

	// Key partitions the call's protection state: a matched rule's identity, or
	// the service name under the default policy. It is bounded by the config, so
	// metadata (unbounded) selects a rule but never becomes a state key.
	Key string

	// Label is the label-level policy — the first rule whose Service matches the
	// executor's own label, or the default. It is what the executor-wide organs
	// (the retry budget) are sized from, since those are per-label by design
	// rather than per-rule.
	Label ClientPolicy

	// Gen is the generation of the rule set behind Policy/Key/Label. An executor
	// whose materialized state was built from a different Gen replaces all of it.
	Gen uint64
}

// PolicySource is what an outbound executor reads to learn the protection one
// call runs under. The manager implements it; an executor built outside a
// manager simply never receives one (see [defaultExecutor.SetPolicySource]),
// and then its construction-time policy governs every call.
//
// Implementations MUST be safe for concurrent use and MUST NOT call back into
// the executor: Resolve runs on the call path, outside the executor's lock.
type PolicySource interface {
	Resolve(op chain.Operation) CallPolicy

	// ResolveLabel answers for the LABEL alone — the label-level policy and its
	// generation, with no rule matching and no per-call key. A backend that keys
	// its own state by resource name (sentinel, which cannot partition one
	// service's protection per rule) reads this instead of building an
	// [chain.Operation] it would then ignore.
	ResolveLabel(label string) CallPolicy
}

// PolicySourceSetter is the construction-time handshake through which an
// outbound executor receives its [PolicySource]. It is the pull-side
// counterpart of [BreakerEventListenerSetter], and it is asserted exactly the
// same way — on the executor the manager itself built, while that executor is
// still private, so no decorator stacked on top of it ever sees the protocol.
//
// A driver whose executor does not implement it keeps the construction-time
// policy for every call: governance still protects, but per-rule matching and
// generation-following are unavailable to that backend.
type PolicySourceSetter interface {
	SetPolicySource(PolicySource)
}

// clientSnapshot is one generation of the outbound settings, published whole by
// [ClientManager.Apply] and read lock-free by every executor's Resolve. The per-label
// generation is memoized here rather than precomputed, because the set of labels
// in use is discovered lazily — a label exists when some client first asks for
// it — so it cannot be enumerated at apply time. A fresh snapshot starts with an
// empty cache, so a memo can never outlive the settings it was derived from.
type clientSnapshot struct {
	settings ClientSettings
	gens     sync.Map // label -> uint64
}

func newClientSnapshot(s ClientSettings) *clientSnapshot {
	return &clientSnapshot{settings: s}
}

// gen returns label's generation, computing it on first use.
func (s *clientSnapshot) gen(label string) uint64 {
	if v, ok := s.gens.Load(label); ok {
		return v.(uint64)
	}
	g := clientLabelGen(label, &s.settings)
	s.gens.Store(label, g)
	return g
}

// resolve is [ClientManager.Resolve]'s body: the matching itself, lock-free over one
// snapshot. One pass over the rules answers both halves — which rule governs this
// call, and what the label resolves to when no rule does — so a call costs one
// scan and one memoized generation lookup.
func (s *clientSnapshot) resolve(op chain.Operation) CallPolicy {
	var (
		label   ClientPolicy // the label-level half: the FIRST rule naming the service
		hasRule bool
	)
	if op.Service != "" {
		for i := range s.settings.Rules {
			r := &s.settings.Rules[i]
			if r.Service != op.Service {
				continue
			}
			// The label-level half is settled by the FIRST rule naming the
			// service, which is reached at or before the rule this call matches —
			// so the first match can return straight away.
			if !hasRule {
				label, hasRule = r.ClientPolicy, true
			}
			if r.Action.Matches(op.Request.Action) && r.Match.Matches(op) {
				return CallPolicy{
					Policy: r.ClientPolicy,
					Key:    ruleKey(r.Service, i),
					Label:  label,
					Gen:    s.gen(op.Service),
				}
			}
		}
	}
	if !hasRule {
		label = s.settings.Default
	}
	return CallPolicy{Policy: s.settings.Default, Key: op.Service, Label: label, Gen: s.gen(op.Service)}
}

// labelPolicy is the label-level resolution: the first rule whose Service equals
// the label, else the default. It matches [ClientManager.policyLocked], and
// deliberately ignores Action/Match — the executor-wide organs are per label,
// not per rule.
func (s *clientSnapshot) labelPolicy(label string) ClientPolicy {
	for _, r := range s.settings.Rules {
		if r.Service == label {
			return r.ClientPolicy
		}
	}
	return s.settings.Default
}

// resolveLabel is [clientSnapshot.resolve] for the label alone, for backends
// that keep one resource per service and can neither match on a call nor
// partition state.
func (s *clientSnapshot) resolveLabel(label string) CallPolicy {
	p := s.labelPolicy(label)
	return CallPolicy{Policy: p, Key: label, Label: p, Gen: s.gen(label)}
}

// clientLabelGen fingerprints everything the outbound side derives for label:
// the default policy (which governs the label when no rule matches) and every
// rule that names it, in order and with its position included — position is part
// of the fingerprint because the state key embeds the rule's index, so a
// reordering changes which state a rule's calls belong to.
//
// Only a label's OWN rules enter its fingerprint, which is what lets a change to
// one service leave every other service's materialized state untouched.
func clientLabelGen(label string, s *ClientSettings) uint64 {
	h := fnv.New64a()
	h.Write([]byte("default\x00"))
	fingerprint(h, fmt.Sprintf("%+v", s.Default))
	for i := range s.Rules {
		r := &s.Rules[i]
		if r.Service != label {
			continue
		}
		h.Write([]byte("rule\x00"))
		fingerprint(h, strconv.Itoa(i))
		fingerprint(h, r.Service)
		fingerprint(h, r.Action.Exact)
		fingerprint(h, r.Action.Prefix)
		fingerprintSelector(h, r.Match)
		fingerprint(h, fmt.Sprintf("%+v", r.ClientPolicy))
	}
	return h.Sum64()
}

// fingerprint writes one field, delimited so that ("ab","c") and ("a","bc")
// cannot collide.
func fingerprint(h hash.Hash64, s string) {
	h.Write([]byte(s))
	h.Write([]byte{0})
}

// fingerprintSelector writes a match selector in key order — a Go map iterates
// nondeterministically, and a generation that changed on every process start
// would rebuild every breaker for no reason.
func fingerprintSelector(h hash.Hash64, sel chain.Selector) {
	if len(sel) == 0 {
		h.Write([]byte{0})
		return
	}
	keys := make([]string, 0, len(sel))
	for k := range sel {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fingerprint(h, k)
		fingerprint(h, sel[k])
	}
}
