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
	"hash"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
)

// SelectionSource is what a [Chain] asks to learn the selection its label
// currently resolves to and which generation that is. The [Manager] implements
// it; the chain reads it on every request and rebuilds when the generation
// moves.
//
// It is the PULL seam: a [Manager.Apply] publishes a new generation and does
// nothing else — no walking of registered chains, no reaching into them. Each
// chain notices on its own next request. It is the same shape the resilience
// layer's executables use (see cloud/resilience's PolicySource), which is what
// lets the two modules be described the same way: a manager that holds the
// config, and a stable shell per resource that replaces the state it
// materialized when the config generation moves.
//
// Implementations MUST be safe for concurrent use and MUST NOT call back into
// the chain.
type SelectionSource interface {
	// ResolveSelection returns the selection label resolves to right now and the
	// generation of the rule set it came from. The generation is derived from the
	// CONTENT of that set, never from a push counter, so a document re-pushed
	// unchanged does not move it.
	ResolveSelection(label string) (Selection, uint64)
}

// labelResolution is what a label resolves to, memoized per snapshot: the
// selection in force and the generation it belongs to.
type labelResolution struct {
	sel Selection
	gen uint64
}

// selectionSnapshot is one generation of the selection document, published whole
// by [Manager.Apply] and read lock-free through [Manager.ResolveSelection].
//
// The per-label ANSWER is memoized here rather than precomputed: labels are
// discovered lazily (a label exists once some client asks for a chain under it),
// so they cannot be enumerated at apply time. Memoizing the whole answer — not
// just the generation — is what keeps the request path down to one map lookup:
// the rules are only scanned and fingerprinted once per label per document. The
// settings are immutable for the snapshot's whole life, so a memo cannot go
// stale, and a fresh snapshot starts with an empty cache.
type selectionSnapshot struct {
	settings Settings
	resolved sync.Map // label -> labelResolution
}

func newSelectionSnapshot(s Settings) *selectionSnapshot {
	return &selectionSnapshot{settings: s}
}

// resolve is [Manager.ResolveSelection]'s body.
func (s *selectionSnapshot) resolve(label string) labelResolution {
	if v, ok := s.resolved.Load(label); ok {
		return v.(labelResolution)
	}
	r := labelResolution{sel: s.selection(label), gen: labelGen(label, &s.settings)}
	s.resolved.Store(label, r)
	return r
}

// selection is the label-level resolution: the first rule whose Service equals
// the label, else the default. Labels are unique across rules (the center
// rejects a duplicate document), so list order cannot decide which entry
// applies.
func (s *selectionSnapshot) selection(label string) Selection {
	for _, r := range s.settings.Rules {
		if r.Service == label {
			return r.Selection
		}
	}
	return s.settings.Default
}

// labelGen fingerprints everything label's selection depends on: the default
// (which governs the label when no rule matches) and the rule that names it,
// with its position included — position is part of the fingerprint because a
// reordering changes which rule a label resolves to.
//
// Only the label's own rule enters its fingerprint, which is what lets a change
// to one service leave every other service's chain untouched.
func labelGen(label string, s *Settings) uint64 {
	h := fnv.New64a()
	h.Write([]byte("default\x00"))
	selectionFingerprint(h, s.Default)
	for i := range s.Rules {
		r := &s.Rules[i]
		if r.Service != label {
			continue
		}
		h.Write([]byte("rule\x00"))
		selectionField(h, strconv.Itoa(i))
		selectionField(h, r.Service)
		selectionFingerprint(h, r.Selection)
	}
	return h.Sum64()
}

// selectionFingerprint writes one selection, delimited so that neighbouring fields cannot
// run together, and with its parameter map in key order — a Go map iterates
// nondeterministically, and a generation that changed on every process start
// would rebuild every chain for no reason.
func selectionFingerprint(h hash.Hash64, s Selection) {
	selectionField(h, s.Balancer)
	selectionField(h, strconv.FormatInt(int64(s.OutlierThreshold), 10))
	selectionField(h, strconv.FormatInt(int64(s.OutlierSuspendFor), 10))
	if len(s.Params) == 0 {
		h.Write([]byte{0})
		return
	}
	keys := make([]string, 0, len(s.Params))
	for k := range s.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		selectionField(h, k)
		selectionField(h, s.Params[k])
	}
}

// selectionField writes one delimited field.
func selectionField(h hash.Hash64, s string) {
	h.Write([]byte(s))
	h.Write([]byte{0})
}
