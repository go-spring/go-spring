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

package StarterNeo4j

import (
	"context"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// fakeDriver is a nil-behavior DriverWithContext so queryResilience can be
// exercised against a *Client wrapper without a live Neo4j server. The guard
// tests never reach the driver: they drive RunWithResilience with a stubbed
// fn and assert the executor short-circuits before fn runs.
type fakeDriver struct {
	neo4j.DriverWithContext
}

// newGuardedNeo4jClient builds a Client whose executor comes from the default
// resilience driver. The embedded driver is nil.
func newGuardedNeo4jClient(t *testing.T, p resilience.Policy) *Client {
	d, err := resilience.GetDriver("default")
	assert.Error(t, err).Nil()
	exec, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	return &Client{DriverWithContext: fakeDriver{}, exec: exec, resource: "neo4j:test"}
}

// TestRunWithResiliencePassThrough proves the zero-config stance: a Client
// with no executor attached (governance off) runs fn inline unchanged, and a
// raw (non-wrapper) driver stays unprotected rather than erroring.
func TestRunWithResiliencePassThrough(t *testing.T) {
	c := &Client{DriverWithContext: fakeDriver{}}
	assert.Error(t, RunWithResilience(context.Background(), c, func(context.Context) error { return nil })).Nil()

	raw := fakeDriver{}
	assert.Error(t, RunWithResilience(context.Background(), raw, func(context.Context) error { return nil })).Nil()
}

// TestRunWithResilienceRateLimit confirms the flow-control path: once the
// burst is spent, fn is rejected without running.
func TestRunWithResilienceRateLimit(t *testing.T) {
	c := newGuardedNeo4jClient(t, resilience.Policy{RateLimit: 1, Burst: 1})
	var ran int
	fn := func(context.Context) error {
		ran++
		return nil
	}
	assert.Error(t, RunWithResilience(context.Background(), c, fn)).Nil()
	assert.Error(t, RunWithResilience(context.Background(), c, fn)).Is(resilience.ErrRateLimited)
	assert.That(t, ran).Equal(1) // the rejected call never ran
}
