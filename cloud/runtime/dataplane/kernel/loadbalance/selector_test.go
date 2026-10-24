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
	"context"
	"testing"

	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/cloud/runtime/dataplane/kernel/matcher"
	"go-spring.org/stdlib/testing/assert"
)

// versionedSource is a source whose endpoints carry a version, the dimension the
// dynamic selector narrows on.
func versionedSource() discovery.Resolver {
	return staticSource(
		discovery.Endpoint{Addr: "a", Healthy: true, Metadata: map[string]string{"version": "v2.1"}},
		discovery.Endpoint{Addr: "b", Healthy: true, Metadata: map[string]string{"version": "v1.9"}},
	)
}

func versionPrefix(v string) matcher.Selector {
	return matcher.Selector{{Key: "version", Op: matcher.Prefix, Value: v}}
}

// TestChainSelectorNarrowsCandidates pins that a selection's selector filters
// the candidate set before the strategy picks: only matching endpoints are ever
// chosen.
func TestChainSelectorNarrowsCandidates(t *testing.T) {
	m := newManager(t)
	m.Apply(context.Background(), Settings{Default: Selection{Selector: versionPrefix("v2.")}})

	c := m.Chain(context.Background(), versionedSource(), "svc:x")
	for range 4 {
		ep, err := c.Pick(context.Background(), PickInfo{})
		assert.Error(t, err).Nil()
		assert.That(t, ep.Addr).Equal("a")
	}
	assert.Number(t, len(c.Selection().Selector)).Equal(1)
}

// TestChainSelectorChangeRebuilds pins the dynamic half: a pushed selector is an
// ordinary selection change, so it moves the label's generation and the rebuilt
// pool picks from the newly narrowed set — no redeploy, no in-place mutation.
func TestChainSelectorChangeRebuilds(t *testing.T) {
	m := newManager(t)
	c := m.Chain(context.Background(), versionedSource(), "svc:x")
	assert.Number(t, c.Generation()).Equal(uint64(1)) // no selector: both endpoints

	m.Apply(context.Background(), Settings{Default: Selection{Selector: versionPrefix("v2.")}})
	assert.Number(t, c.Generation()).Equal(uint64(2))
	ep, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr).Equal("a")

	m.Apply(context.Background(), Settings{Default: Selection{Selector: versionPrefix("v1.")}})
	assert.Number(t, c.Generation()).Equal(uint64(3))
	ep, err = c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr).Equal("b")
}

// TestChainSelectorMatchingNothingFailsLoudly pins the no-fallback decision: a
// rule that matches no endpoint surfaces ErrNoAvailable instead of silently
// widening to every instance.
func TestChainSelectorMatchingNothingFailsLoudly(t *testing.T) {
	m := newManager(t)
	m.Apply(context.Background(), Settings{Default: Selection{Selector: versionPrefix("v9.")}})

	c := m.Chain(context.Background(), versionedSource(), "svc:x")
	_, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Is(ErrNoAvailable)
}

// TestValidateRejectsBadSelector pins that a malformed selector is rejected by
// the document validation, including when the rule names no strategy (the base
// strategy is a default, not a reason to skip the rest of the policy). The
// malformation used here is the cross-field one Validate owns: a non-exact op
// with an empty operand (the key/op single-field rules live in the tags).
func TestValidateRejectsBadSelector(t *testing.T) {
	bad := matcher.Selector{{Key: "version", Op: matcher.Prefix, Value: ""}}

	assert.Error(t, Validate(Settings{Default: Selection{Selector: bad}})).NotNil()
	assert.Error(t, Validate(Settings{Rules: []Rule{{Service: "svc:x", Selection: Selection{Selector: bad}}}})).NotNil()
	assert.Error(t, Validate(Settings{Default: Selection{Selector: versionPrefix("v2.")}})).Nil()
}
