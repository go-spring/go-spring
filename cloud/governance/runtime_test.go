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

package governance

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/observe"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/cloud/mesh"
	"go-spring.org/stdlib/testing/assert"
)

// TestUsesDiscovery pins the mesh precedence: a configured service name means
// in-process discovery, except in mesh mode, where the sidecar owns discovery
// and load balancing and the service name is simply ignored (not an error).
func TestUsesDiscovery(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		serviceName string
		want        bool
	}{
		{"configured, no mesh", "off", "orders", true},
		{"unconfigured, no mesh", "off", "", false},
		{"configured, mesh wins", "on", "orders", false},
		{"unconfigured, mesh", "on", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(mesh.ModeEnv, tc.mode)
			r := &Runtime{}
			if got := r.UsesDiscovery(tc.serviceName); got != tc.want {
				t.Fatalf("UsesDiscovery(%q) with mesh mode %q = %v, want %v", tc.serviceName, tc.mode, got, tc.want)
			}
		})
	}
}

// TestExecutorFor_FaultFiresInsideTheRetryLoop pins the composition order. The
// injected failure is retried (one round per attempt) and never reaches fn,
// which only holds when fault is the innermost layer and resilience's loop runs
// it: with fault outside the loop the call would return after the first fire
// and the record would hold no rounds.
func TestExecutorFor_FaultFiresInsideTheRetryLoop(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.Error(t, err).Nil()
	res, err := resilience.NewManager(factories)
	assert.That(t, err).Nil()
	t.Cleanup(func() { _ = res.Close() })
	assert.Error(t, res.ApplyTimeoutRetry(resilience.TimeoutRetrySettings{Services: []resilience.TimeoutRetryService{{
		Service: "svc",
		Rules: []resilience.TimeoutRetryRule{{
			ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			TimeoutRetry: resilience.TimeoutRetry{
				Retry: resilience.Retry{MaxRetries: 2},
			},
		}},
	}}})).Nil()

	inj := clientfault.NewManager()
	inj.Refresh(clientfault.ClientConfig{Rules: []clientfault.ClientRule{{
		RuleCore: kfault.RuleCore{Name: "fire", Service: "svc", Rate: 1},
		Class:    kfault.ClassTransient, Error: "generic",
	}}})

	r := &Runtime{Resilience: res, ClientFault: inj}
	exec := r.ExecutorFor("test", "svc", nil)
	rec := &chain.Recorder{}
	var calls atomic.Int32
	err = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, rec,
		func(context.Context, *chain.Round) error {
			calls.Add(1)
			return nil
		})
	assert.That(t, kfault.IsInjected(err)).True()
	assert.That(t, calls.Load()).Equal(int32(0))
	assert.That(t, len(rec.Rounds)).Equal(3)
}

// countingMetrics records how many calls the runtime's executor reported
// through the injected metric seam.
type countingMetrics struct {
	calls int
}

func (c *countingMetrics) Inflight(context.Context, observe.Call, int64) {}

func (c *countingMetrics) Call(context.Context, observe.Call, string, chain.ErrorType, time.Duration) {
	c.calls++
}

func (c *countingMetrics) Attempt(context.Context, observe.Call, string, chain.ErrorType, time.Duration) {
}

func (c *countingMetrics) Fault(context.Context, observe.Call, observe.Fault) {}

// TestExecutorFor_UsesInjectedMetrics proves the client emit layer reports
// through the injected seam rather than the default OTel metrics.
func TestExecutorFor_UsesInjectedMetrics(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.Error(t, err).Nil()
	res, err := resilience.NewManager(factories)
	assert.That(t, err).Nil()
	t.Cleanup(func() { _ = res.Close() })

	m := &countingMetrics{}
	r := &Runtime{Resilience: res, ClientFault: clientfault.NewManager(), ClientMetrics: m}
	_exec := r.ExecutorFor("test", "svc", nil)
	err = _exec.Execute(context.Background(), chain.Operation{}, new(chain.Recorder),
		func(context.Context, *chain.Round) error { return nil })
	assert.Error(t, err).Nil()
	assert.Number(t, m.calls).Equal(1)
}
