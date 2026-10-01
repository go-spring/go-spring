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

package StarterCassandra

import (
	"context"
	"errors"
	"testing"

	"github.com/gocql/gocql"
	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// TestParseConsistency covers the config-string mapping, including the
// default and the rejection of unknown values.
func TestParseConsistency(t *testing.T) {
	cases := map[string]gocql.Consistency{
		"":             gocql.LocalQuorum,
		"local-quorum": gocql.LocalQuorum,
		"local-one":    gocql.LocalOne,
		"one":          gocql.One,
		"quorum":       gocql.Quorum,
		"all":          gocql.All,
		"any":          gocql.Any,
		"two":          gocql.Two,
		"three":        gocql.Three,
		"each-quorum":  gocql.EachQuorum,
	}
	for s, want := range cases {
		got, err := parseConsistency(s)
		assert.Error(t, err).Nil()
		assert.That(t, got).Equal(want)
	}
	_, err := parseConsistency("bogus")
	assert.That(t, err != nil).True()
}

// --- guard (transparent per-statement resilience via the Query wrapper) ---

// newGuardedClient builds a Client whose guard is wired to a real executor
// from the default resilience driver, wrapped by the resilience observe layer
// ([resilience.WrapClientExecutor]) exactly as the container wiring does — the
// guard declares the operation and this wrapper emits from it. The embedded
// *gocql.Session is nil — the tests drive Client.guard directly with a stubbed
// call, so no live Cassandra cluster is needed.
func newGuardedClient(t *testing.T, p resilience.ClientPolicy) *Client {
	d := resilience.NewDefaultDriver(nil)
	inner, err := d.NewClientExecutor("svc", p)
	assert.Error(t, err).Nil()
	exec := resilience.WrapClientExecutor(inner, "cassandra", "cassandra:test")
	return &Client{exec: exec, serviceLabel: "cassandra:test"}
}

// TestGuardPassThrough proves the degraded stance: a Client assembled without a
// container (the zero governance bundle) still runs the call — its executor is
// the observed-only resilience.Unmanaged, which applies no rate limit, breaker
// or retry — so the call runs inline and returns its result unchanged.
func TestGuardPassThrough(t *testing.T) {
	c := NewClient(nil, Config{Hosts: []string{"127.0.0.1"}}, cloud.ClientParams{})
	boom := errors.New("boom")
	assert.Error(t, c.guard(context.Background(), "exec", "SELECT 1", func(context.Context) error { return nil })).Nil()
	assert.Error(t, c.guard(context.Background(), "exec", "SELECT 1", func(context.Context) error { return boom })).Is(boom)
}

// TestGuardRateLimit confirms the flow-control path: once the burst is spent,
// the statement is rejected without invoking the call.
func TestGuardRateLimit(t *testing.T) {
	c := newGuardedClient(t, resilience.ClientPolicy{RateLimit: 1, Burst: 1})
	var ran int
	stub := func(context.Context) error {
		ran++
		return nil
	}
	assert.Error(t, c.guard(context.Background(), "exec", "INSERT INTO t VALUES(1)", stub)).Nil()
	assert.Error(t, c.guard(context.Background(), "exec", "INSERT INTO t VALUES(2)", stub)).Is(resilience.ErrRateLimited)
	assert.That(t, ran).Equal(1) // the rejected statement never ran
}
