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
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestDirectoryBuildsBuiltins(t *testing.T) {
	dir := builtinDirectory()
	for _, name := range []string{RoundRobin, LeastConn, ConsistentHash, Weighted, ZoneAware, Random, P2C} {
		b, err := dir.Build(name, NewParams(nil))
		assert.Error(t, err).Nil()
		assert.That(t, b).NotNil()
	}

	_, err := dir.Build("does-not-exist", NewParams(nil))
	assert.Error(t, err).Matches("no strategy registered")

	assert.That(t, dir.Names()).Equal([]string{
		ConsistentHash, LeastConn, P2C, Random, RoundRobin, Weighted, ZoneAware,
	})
}

// TestDirectoryContribution covers the extension point: a deployment adds a
// strategy by contributing a named Factory, and a name that shadows a built-in
// is refused rather than silently winning.
func TestDirectoryContribution(t *testing.T) {
	dir, err := NewDirectory(map[string]Factory{"mine": stubFactory{}})
	assert.Error(t, err).Nil()
	assert.That(t, dir.Names()).Equal([]string{
		ConsistentHash, LeastConn, "mine", P2C, Random, RoundRobin, Weighted, ZoneAware,
	})
	if _, err := dir.Build("mine", NewParams(nil)); err != nil {
		t.Fatalf("contributed strategy not built: %v", err)
	}

	_, err = NewDirectory(map[string]Factory{RoundRobin: stubFactory{}})
	assert.Error(t, err).Matches("shadows a built-in")

	_, err = NewDirectory(map[string]Factory{"": stubFactory{}})
	assert.Error(t, err).Matches("empty name")

	_, err = NewDirectory(map[string]Factory{"nil": nil})
	assert.Error(t, err).Matches("nil factory")
}

// TestParamsPartition covers the strict-partition guarantee carried over from
// the typed Config: a parameter aimed at another strategy is rejected at
// construction, not silently dropped.
func TestParamsPartition(t *testing.T) {
	dir := builtinDirectory()

	_, err := dir.Build(LeastConn, NewParams(map[string]string{ParamReplicas: "200"}))
	assert.Error(t, err).Matches("not understood by this strategy")

	_, err = dir.Build(ConsistentHash, NewParams(map[string]string{ParamZoneKey: "zone"}))
	assert.Error(t, err).Matches("not understood by this strategy")

	// A strategy's own parameters pass; the empty bag passes everywhere.
	if _, err := dir.Build(ConsistentHash, NewParams(map[string]string{ParamReplicas: "200"})); err != nil {
		t.Fatal(err)
	}
	if _, err := dir.Build(ZoneAware, NewParams(map[string]string{ParamZoneKey: "zone", ParamDelegate: LeastConn})); err != nil {
		t.Fatal(err)
	}

	// A present-but-unparsable value is an error, never a silent default.
	_, err = dir.Build(ConsistentHash, NewParams(map[string]string{ParamReplicas: "many"}))
	assert.Error(t, err).Matches("not an integer")

	// zone_aware delegate errors: unknown name, and self-delegation.
	_, err = dir.Build(ZoneAware, NewParams(map[string]string{ParamDelegate: "nope"}))
	assert.Error(t, err).Matches("zone_aware delegate")
	_, err = dir.Build(ZoneAware, NewParams(map[string]string{ParamDelegate: ZoneAware}))
	assert.Error(t, err).Matches("cannot delegate to itself")
}

// stubFactory is a minimal contributed strategy, used only to exercise the
// directory's own rules.
type stubFactory struct{}

func (stubFactory) Build(_ Directory, p *Params) (Balancer, error) {
	if err := p.Done(); err != nil {
		return nil, err
	}
	return NewRoundRobin(), nil
}
