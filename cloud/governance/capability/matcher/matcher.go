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
// the subset-match rules that endpoint selection, policy and fault injection
// all evaluate against a string map.
//
// The model is the intersection of what mainstream metadata filters offer; it
// deliberately omits an expression language and key patterns. Keys are
// enumerated — a key is a governed name, and a key pattern would make a rule
// impossible to validate, audit, or push down. Each entry names exactly one
// comparison, and entries are ANDed with no boolean combinator, because "or"
// belongs one level up (several rules) or inside a value set. That keeps a rule
// monotone: adding an entry can only narrow it, so a control plane can treat
// "the rule got wider" as an event it caused on purpose.
//
// Matching is subset-match: the metadata may carry entries the rule does not
// name, but a named key must be PRESENT and satisfy its matcher.
//
// [StringMatch] is both the document binding shape and the runtime shape — the
// Istio-headers design, `match.tenant.exact=acme`: one key carries one
// condition and the op is always spelled out, because documents are
// platform-generated rather than hand-written, so there is no scalar
// shorthand. There is no second runtime vocabulary to translate to; [MetadataMatches]
// evaluates the map as bound.
package matcher

import (
	"fmt"
	"strings"
)

// StringMatch is one metadata entry's matcher — the shape a document writes
// for a key: name the op, give it a value (`tenant: {exact: acme}`,
// `version: {prefix: v2}`). The op set is closed — exact, prefix, suffix,
// contains — and one key carries exactly one condition, so a document cannot
// leave the op implicit or stack two conditions on one key.
//
// Exactly one field must be set; [StringMatch.Validate] enforces that at the
// source boundary, so a malformed condition never reaches the request path.
type StringMatch struct {
	// Exact matches the metadata value equal to it. An empty Exact is
	// indistinguishable from unset at binding, so a present-but-empty VALUE
	// cannot be targeted (see [StringMatch.Validate]).
	Exact string `value:"${exact:=}" json:"exact"`

	// Prefix matches a value starting with it. Empty does not bind, so the
	// degenerate "matches everything" condition cannot be written.
	Prefix string `value:"${prefix:=}" json:"prefix"`

	// Suffix matches a value ending with it. Empty does not bind (see Prefix).
	Suffix string `value:"${suffix:=}" json:"suffix"`

	// Contains matches a value containing it. Empty does not bind (see Prefix).
	Contains string `value:"${contains:=}" json:"contains"`
}

// Validate checks the one rule a per-field expr cannot express: exactly one op
// is set. An all-empty value (nothing written, or an operand the document
// tried to leave empty) or two set ops is a malformed document, rejected where
// it is read. conf.Bind calls this, so a malformed condition never reaches the
// request path.
func (m StringMatch) Validate() error {
	set := 0
	for _, v := range []string{m.Exact, m.Prefix, m.Suffix, m.Contains} {
		if v != "" {
			set++
		}
	}
	switch {
	case set == 0:
		return fmt.Errorf("matcher: string match sets no op")
	case set > 1:
		return fmt.Errorf("matcher: string match sets more than one op")
	}
	return nil
}

// Matches reports whether v satisfies m. An unset op (a value that bypassed
// [StringMatch.Validate]) matches nothing: failing closed is the only safe
// answer for a condition that slipped past the boundary.
func (m StringMatch) Matches(v string) bool {
	switch {
	case m.Exact != "":
		return v == m.Exact
	case m.Prefix != "":
		return strings.HasPrefix(v, m.Prefix)
	case m.Suffix != "":
		return strings.HasSuffix(v, m.Suffix)
	case m.Contains != "":
		return strings.Contains(v, m.Contains)
	default:
		return false
	}
}

// MetadataMatches reports whether metadata satisfies every entry of match — a
// conjunction over one metadata map, subset-match: the metadata may carry
// entries match does not name, but a named key must be present and satisfy its
// [StringMatch]. The empty map matches everything; because that is
// indistinguishable from a dropped rule, a control-plane decoder should reject
// an explicitly empty map as a configuration error instead of installing it as
// "match all". Evaluation allocates nothing, so there is nothing to precompute.
func MetadataMatches(match map[string]StringMatch, metadata map[string]string) bool {
	for k, m := range match {
		v, ok := metadata[k]
		if !ok || !m.Matches(v) {
			return false
		}
	}
	return true
}

// ActionMatch is a rule's match condition over a call's action — the interface
// being invoked ("insert", "GET /users/{id}"). It is the single-value
// counterpart of the map vocabulary above: the "which action" question a policy
// rule and a fault rule both ask, shared here because the two layers must not
// derive it from each other. Its zero value matches every action, so a rule
// that declares no action condition governs the whole service.
//
// Exact is equality; Prefix matches a leading string, which is how one rule
// covers a REST subtree whose concrete paths vary ("GET /users" covers
// "GET /users/{id}"). They are mutually exclusive in practice — Exact wins when
// both are set — so a rule states one intent.
//
// WHY SO SHALLOW: fault injection and policy targeting are not routing. A
// router needs rich matching because forwarding to the wrong backend is an
// incident; a rule needs matching an operator can predict, because the first
// question during a drill is "what exactly did we just hit". The axis a rule
// usually needs beyond this is the dose, not a richer predicate.
//
// EXTENSION: operators are added as NEW FIELDS under the same one-of
// convention, never by reinterpreting an existing field, so every config
// written today keeps its meaning. A regex operator would earn its place only
// when a real case needs to split one service by path pattern that Prefix
// cannot express; it would then be added here (compiled once per distinct
// pattern, not per call), not as a "match strategy" enum on an existing field.
type ActionMatch struct {
	Exact  string `value:"${exact:=}"`
	Prefix string `value:"${prefix:=}"`
}

// Matches reports whether action satisfies m. The service marker matches every
// action; otherwise the first non-empty operator wins, and the zero value
// matches every action.
func (m ActionMatch) Matches(action string) bool {
	switch {
	case m.Exact == ServiceScopeAction:
		return true
	case m.Exact != "":
		return action == m.Exact
	case m.Prefix != "":
		return strings.HasPrefix(action, m.Prefix)
	default:
		return true
	}
}

// ServiceScopeAction is the reserved Exact selector that means "the whole
// service". It is the one selector a real action can never spell, so the marker
// cannot be confused with an interface.
const ServiceScopeAction = "-"

// IsServiceScope reports whether m selects the whole service (the reserved
// [ServiceScopeAction] marker).
func (m ActionMatch) IsServiceScope() bool {
	return m.Exact == ServiceScopeAction
}

// BetterThan reports whether m is the more specific ACTION selector of
// the two: exact beats any prefix, a longer prefix beats a shorter one, and a
// real action beats the service scope. Equal selectors do not outrank each
// other. Metadata is a separate dimension, composed by the caller (see
// [MetadataMatches]).
func (m ActionMatch) BetterThan(o ActionMatch) bool {
	switch {
	case m.Exact == ServiceScopeAction:
		// The whole-service fallback beats nothing.
		return false
	case o.Exact == ServiceScopeAction:
		// Any real selector beats the whole-service fallback.
		return true
	case m.Exact != "" && o.Exact != "":
		// Two exact selectors: neither outranks the other.
		return false
	case m.Exact != "":
		// Exact beats prefix.
		return true
	case o.Exact != "":
		// Prefix loses to exact.
		return false
	default:
		// Two prefixes (the zero selector is an empty prefix): longer wins.
		return len(m.Prefix) > len(o.Prefix)
	}
}
