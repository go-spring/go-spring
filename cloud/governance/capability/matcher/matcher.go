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

// Package matcher is the ONE metadata-matching vocabulary of the data plane:
// the subset-match rules that endpoint selection, policy and fault
// injection all evaluate against a string map.
//
// The model is the intersection of what mainstream metadata filters offer; it
// deliberately omits an expression language and key patterns. Keys are
// enumerated — a key is a governed name, and a key pattern would make a rule
// impossible to validate, audit, or push down. Each term names exactly one comparison ([Op]); terms are
// ANDed with no boolean combinator, because "or" belongs one level up (several
// rules) or inside a value set. That keeps a rule monotone: adding a term can
// only narrow it, so a control plane can treat "the rule got wider" as an event
// it caused on purpose.
//
// Matching is subset-match: the metadata may carry entries the rule does not
// name, but a named key must be PRESENT and satisfy its term. Presence is part
// of the rule — [Exact] with an empty operand matches an empty value, not a
// missing key — so `exact: ""` and "the key is absent" never collapse.
//
// Configuration has two binding shapes because the syntax differs while the
// semantics must not: [Selector] is the typed term list (the only shape that
// expresses more than equality), and [ExactMap] is the exact-only map read as
// `match.tenant=acme`. An [ExactMap] is exactly the [Selector] built from the
// same pairs with [Exact].
package matcher

import (
	"fmt"
	"strings"
)

// Op is the comparison a [Term] applies to one value. It is a closed set, not
// an expression language: an unknown op is rejected by the field's expr at bind
// time, so a malformed rule never reaches the request path.
//
// It is called Op rather than Kind because Kind already names the interaction
// shape of a call (call, publish, consume) in this codebase; a comparison is an
// operator, and reusing the word for both would make the two vocabularies read
// as one.
type Op string

const (
	// Exact requires the metadata value to equal [Term.Value].
	Exact Op = "exact"

	// Prefix requires the metadata value to start with [Term.Value].
	Prefix Op = "prefix"

	// Suffix requires the metadata value to end with [Term.Value].
	Suffix Op = "suffix"

	// Contains requires the metadata value to contain [Term.Value].
	Contains Op = "contains"
)

// Term is one metadata condition: the entry under Key must exist, and its value
// must satisfy the comparison named by Op against Value.
//
// A term is a pure value — no closures, no compiled state — so the same rule
// can be written in code or decoded from a control-plane payload and applied
// verbatim. That is also why regex matching is not here yet: it needs a
// compile-once representation, which would either make Term non-serializable or
// compile per evaluation on the request path.
type Term struct {
	// Key is the metadata entry to inspect. Required and non-empty: a term
	// without a name would match nothing, so the field is bound strictly rather
	// than defaulted to "" and failing silently.
	Key string `value:"${key}" json:"key" expr:"$ != ''"`

	// Op is the comparison applied to Key's value. Required, and checked at bind
	// time against the closed set, so a typo fails where the document is read
	// rather than becoming a term that never matches. The expression needs
	// string($) because expr does not compare the named type to a string
	// literal directly.
	//
	// The wire name stays `kind`: rule documents already spell `kind:`, so only
	// the Go name changed.
	Op Op `value:"${kind}" json:"kind" expr:"string($) in ['exact', 'prefix', 'suffix', 'contains']"`

	// Value is the operand of Op. Required at bind time: omitting it is almost
	// always a mistake, and the one meaningful empty operand — [Exact] matching
	// an empty metadata value — must be written out (`value: ""`) rather than
	// arrived at by omission. An empty operand for prefix, suffix or contains
	// holds for every value and would degenerate the term into "the key exists",
	// so [Selector.Validate] rejects it.
	Value string `value:"${value}" json:"value"`
}

// Matches reports whether v satisfies t. An unknown Op matches nothing: the
// field tag rejects one at bind time, and failing closed is the only safe
// answer for a term that bypassed that boundary.
func (t Term) Matches(v string) bool {
	switch t.Op {
	case Exact:
		return v == t.Value
	case Prefix:
		return strings.HasPrefix(v, t.Value)
	case Suffix:
		return strings.HasSuffix(v, t.Value)
	case Contains:
		return strings.Contains(v, t.Value)
	}
	return false
}

// Selector is a conjunction of [Term]s over one metadata map.
//
// The empty selector matches everything. Because that is indistinguishable from
// a dropped rule, a control-plane decoder should reject an explicitly empty
// selector as a configuration error instead of installing it as "match all".
type Selector []Term

// Matches reports whether metadata satisfies every term in s.
func (s Selector) Matches(metadata map[string]string) bool {
	for _, t := range s {
		v, ok := metadata[t.Key]
		if !ok || !t.Matches(v) {
			return false
		}
	}
	return true
}

// Validate checks the one rule a per-field expr cannot express: an empty
// operand is legal only for [Exact], because empty prefix/suffix/contains holds
// for every value and would degenerate the term into "the key exists".
//
// It re-checks neither the key nor the op: both are single-field rules owned by
// the field tags ([Term.Key], [Term.Op]), and a second check here would give
// each rule two owners.
//
// Repeating a key is allowed — terms are a conjunction, so two terms on version
// (a prefix and a suffix, say) must both hold; repetition can only narrow, never
// widen. Call this once when a rule is applied: on error, keep serving the
// previous rule rather than installing one that matches differently from what
// was written.
func (s Selector) Validate() error {
	for i, t := range s {
		switch t.Op {
		case Prefix, Suffix, Contains:
			if t.Value == "" {
				return fmt.Errorf("matcher: selector term %d (%q): empty %s value", i, t.Key, t.Op)
			}
		}
	}
	return nil
}

// ExactMap is the exact-only subset-match shape: every named key must be
// present in the metadata with the given value. The empty map matches
// everything.
//
// It exists because a rule config reads better as a map (`match.tenant=acme`)
// than as an indexed term list, not because the semantics differ: it is exactly
// the [Selector] built from the same pairs with [Exact]. Configuration binding
// drops an empty key, so the type carries no validation of its own.
type ExactMap map[string]string

// Matches reports whether metadata satisfies every key/value pair in m.
func (m ExactMap) Matches(metadata map[string]string) bool {
	for k, want := range m {
		v, ok := metadata[k]
		if !ok || v != want {
			return false
		}
	}
	return true
}
