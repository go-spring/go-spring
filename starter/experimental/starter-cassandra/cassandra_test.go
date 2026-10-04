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
	"go-spring.org/cloud/chain"
	"testing"

	"github.com/gocql/gocql"
	"go-spring.org/cloud"
	"go-spring.org/cloud/observability"
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

// --- chain (transparent per-statement resilience via the per-query chain) ---

// fakeTail is a scripted InnerQuery tail: it records the context's declared
// operation and answers from a settable error. It stands in for the raw adapter
// so chain tests need no live Cassandra cluster.
type fakeTail struct {
	ops []string
	err error
}

func (f *fakeTail) Exec(ctx context.Context) error {
	f.ops = append(f.ops, opOf(ctx))
	return f.err
}

func (f *fakeTail) Iter(context.Context) *gocql.Iter              { panic("unexpected") }
func (f *fakeTail) Scan(context.Context, ...any) error            { panic("unexpected") }
func (f *fakeTail) ScanCAS(context.Context, ...any) (bool, error) { panic("unexpected") }
func (f *fakeTail) MapScan(context.Context, map[string]any) error { panic("unexpected") }
func (f *fakeTail) MapScanCAS(context.Context, map[string]any) (bool, error) {
	panic("unexpected")
}
func (f *fakeTail) Release(bool) error { return nil }

// opOf reads the operation the identity layer declared onto the context: the
// db.operation attribute from the declared Attrs.
func opOf(ctx context.Context) string {
	op, ok := observability.OperationFrom(ctx)
	if !ok {
		return ""
	}
	for _, kv := range op.Attrs {
		if string(kv.Key) == "db.operation" {
			return kv.Value.AsString()
		}
	}
	return ""
}

// newGuardedExec builds an executor from the default resilience driver, wrapped
// by the resilience observe layer exactly as the container wiring does.
func newGuardedExec(t *testing.T, p resilience.ClientPolicy) chain.Executor {
	d := resilience.NewDefaultDriver(nil)
	inner, err := d.NewClientExecutor("svc", p)
	assert.Error(t, err).Nil()
	return observability.WrapClientExecutor(inner, "cassandra", "cassandra:test")
}

// TestGuardPassThrough proves the degraded stance: a chain assembled without a
// container (the zero governance bundle) still runs the call — its executor is
// the observed-only resilience.Unmanaged, which applies no rate limit, breaker
// or retry — so the call runs inline and returns its result unchanged, with the
// identity layer's declaration visible to the tail.
func TestGuardPassThrough(t *testing.T) {
	c := NewClient(nil, Config{Hosts: []string{"127.0.0.1"}}, cloud.ClientParams{})
	tail := &fakeTail{}
	head := NewObsQuery(NewGuardQuery(tail, c.exec), "SELECT 1")

	boom := errors.New("boom")
	assert.Error(t, head.Exec(context.Background())).Nil()
	assert.That(t, tail.ops).Equal([]string{"exec"})
	tail.err = boom
	assert.Error(t, head.Exec(context.Background())).Is(boom)
}

// TestGuardRateLimit confirms the flow-control path: once the burst is spent,
// the statement is rejected without reaching the tail.
func TestGuardRateLimit(t *testing.T) {
	c := &Client{exec: newGuardedExec(t, resilience.ClientPolicy{RateLimit: 1, Burst: 1})}
	tail := &fakeTail{}
	head := NewObsQuery(NewGuardQuery(tail, c.exec), "INSERT INTO t VALUES(?)")

	assert.Error(t, head.Exec(context.Background())).Nil()
	assert.Error(t, head.Exec(context.Background())).Is(chain.ErrRateLimited)
	assert.That(t, len(tail.ops)).Equal(1) // the rejected statement never ran
}

// rewriteLayer wraps a chain head and rewrites the statement's identity — the
// kind of behavior change no gocql middleware could express.
type rewriteLayer struct {
	InnerQuery
	stmt string
}

func (r rewriteLayer) Exec(ctx context.Context) error {
	return r.InnerQuery.Exec(observability.WithOperation(ctx, operation("exec", r.stmt)))
}

// TestInnerQueryReorganize pins the wrap-head protocol on the builder: a custom
// layer over the query's chain head rewrites what the identity layer below it
// declares, and the terminal method runs through it.
func TestInnerQueryReorganize(t *testing.T) {
	c := NewClient(nil, Config{Hosts: []string{"127.0.0.1"}}, cloud.ClientParams{})
	tail := &fakeTail{}
	head := NewObsQuery(NewGuardQuery(tail, c.exec), "SELECT original")

	wrapped := rewriteLayer{InnerQuery: head, stmt: "SELECT rewritten"}
	assert.Error(t, wrapped.Exec(context.Background())).Nil()
	assert.That(t, tail.ops).Equal([]string{"exec"})
}
