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

	"go-spring.org/cloud/governance/capability/traffic"
	admissionplane "go-spring.org/cloud/governance/controlplane/admission"
	loadbalanceplane "go-spring.org/cloud/governance/controlplane/loadbalance"
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	"github.com/gocql/gocql"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	observability "go-spring.org/cloud/governance/dataplane/outbound/observe"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// testProp is the process traffic convention these fixtures run under.
var testProp = traffic.NewDefaultPropagator(traffic.DefaultBinding())

// newTestCenter builds a runtime over an unarmed resilience manager
// and an empty fault injector — the container-supplied facilities a Client is
// assembled with.
func newTestCenter(t *testing.T) *governance.Runtime {
	t.Helper()
	ctr := &governance.Runtime{
		Resilience:  resilience.NewManager(resilience.NewDefaultDriver(nil)),
		LoadBalance: loadbalance.NewManager(),
		Admission:   admission.NewManager(),
		ClientFault: clientfault.NewManager(),
		ServerFault: serverfault.NewManager(),
		Directory:   discovery.NewManager(nil),
		Traffic:     traffic.NewDefaultPropagator(traffic.DefaultBinding()),
	}
	if _, err := loadbalanceplane.NewCenter(context.Background(), ctr.LoadBalance, source.NewPushSource(loadbalance.Settings{})); err != nil {
		t.Fatal(err)
	}
	if _, err := admissionplane.NewCenter(context.Background(), ctr.Admission, source.NewPushSource(admission.Settings{})); err != nil {
		t.Fatal(err)
	}
	return ctr
}

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

// fakeTail is a scripted InnerQuery tail: it counts executions and answers from
// a settable error. It stands in for the raw adapter so chain tests need no
// live Cassandra cluster.
type fakeTail struct {
	execs int
	err   error
}

func (f *fakeTail) Exec(context.Context) error {
	f.execs++
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

// opName reads the db.operation attribute of an operation the guarded layer
// passed explicitly to the executor.
func opName(op chain.Operation) string {
	for _, kv := range op.Observe.Metric.Attrs {
		if string(kv.Key) == "db.operation" {
			return kv.Value.AsString()
		}
	}
	return ""
}

// captureExec records the operation each call carries and delegates to the
// wrapped executor, so the chain tests can assert the identity reaches the
// executor explicitly rather than through the context.
type captureExec struct {
	next chain.Executor
	ops  []string
}

func (c *captureExec) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	c.ops = append(c.ops, opName(op))
	return c.next.Execute(ctx, op, rec, fn)
}
func (c *captureExec) Close() error { return c.next.Close() }

// newGuardedExec builds an executor from the default resilience driver, wrapped
// by the resilience observe layer exactly as the container wiring does.
func newGuardedExec(t *testing.T, p resilience.RateLimitPolicy) chain.Executor {
	m := resilience.NewManager(resilience.NewDefaultDriver(nil))
	assert.Error(t, m.ApplyRateLimit(resilience.RateLimitSettings{Default: p})).Nil()
	inner := m.ExecutorFor("svc")
	return observability.WrapClientExecutor(inner, "cassandra", "cassandra:test")
}

// TestGuardPassThrough proves the pass-through stance: a chain over an unarmed
// manager still runs the call — its executor applies no rate limit, breaker or
// retry — so the call runs inline and returns its result unchanged, with the
// guarded layer's identity passed explicitly to the executor.
func TestGuardPassThrough(t *testing.T) {
	c := NewClient(nil, Config{Hosts: []string{"127.0.0.1"}}, newTestCenter(t))
	tail := &fakeTail{}
	cap := &captureExec{next: c.exec}
	head := NewGuardQuery(tail, cap, "cassandra:test", "SELECT 1", testProp)

	boom := errors.New("boom")
	assert.Error(t, head.Exec(context.Background())).Nil()
	assert.That(t, cap.ops).Equal([]string{"exec"})
	assert.That(t, tail.execs).Equal(1)
	tail.err = boom
	assert.Error(t, head.Exec(context.Background())).Is(boom)
}

// TestGuardRateLimit confirms the flow-control path: once the burst is spent,
// the statement is rejected without reaching the tail.
func TestGuardRateLimit(t *testing.T) {
	cap := &captureExec{next: newGuardedExec(t, resilience.RateLimitPolicy{RateLimit: 1, Burst: 1})}
	tail := &fakeTail{}
	head := NewGuardQuery(tail, cap, "cassandra:test", "INSERT INTO t VALUES(?)", testProp)

	assert.Error(t, head.Exec(context.Background())).Nil()
	assert.Error(t, head.Exec(context.Background())).Is(chain.ErrRateLimited)
	assert.That(t, cap.ops).Equal([]string{"exec", "exec"}) // identity reaches the executor on every call
	assert.That(t, tail.execs).Equal(1)                     // the rejected statement never ran
}

// passthroughLayer wraps a chain head and delegates — the wrap-head protocol on
// the builder, which now carries no identity of its own (the guarded layer below
// builds and passes it to the executor).
type passthroughLayer struct {
	InnerQuery
}

func (p passthroughLayer) Exec(ctx context.Context) error { return p.InnerQuery.Exec(ctx) }

// TestInnerQueryReorganize pins the wrap-head protocol on the builder: a custom
// layer over the query's chain head still runs the terminal method through the
// guarded chain below it, whose identity reaches the executor.
func TestInnerQueryReorganize(t *testing.T) {
	c := NewClient(nil, Config{Hosts: []string{"127.0.0.1"}}, newTestCenter(t))
	tail := &fakeTail{}
	cap := &captureExec{next: c.exec}
	head := NewGuardQuery(tail, cap, "cassandra:test", "SELECT original", testProp)

	wrapped := passthroughLayer{InnerQuery: head}
	assert.Error(t, wrapped.Exec(context.Background())).Nil()
	assert.That(t, cap.ops).Equal([]string{"exec"})
	assert.That(t, tail.execs).Equal(1)
}
