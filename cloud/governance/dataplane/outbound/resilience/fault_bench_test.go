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

// fault_bench_test.go measures what the injector costs ON TOP of this layer, in
// the production composition (resilience outside, fault inside). It lives here
// so the fault package's tests never reach back into this one.

package resilience

import (
	"context"
	"testing"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/fault"
)

// noopFn is the operation handed to the executor: returns nil immediately so
// the benchmark isolates framework overhead, not downstream work.
func noopFn(context.Context) error { return nil }

// benchExec builds the policy-free executor for "svc".
func benchExec(b *testing.B) chain.Executor {
	b.Helper()
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		b.Fatal(err)
	}
	m, err := NewManager(factories)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = m.Close() })
	return m.Wrap("svc", nil)
}

// faultWrapped puts the injector around raw, armed with conf.
func faultWrapped(raw chain.Executor, conf kfault.Config) chain.Executor {
	inj := fault.NewManager()
	inj.Refresh(conf)
	return inj.WrapClientExecutor(raw)
}

// BenchmarkExecutor_Baseline measures the raw default executor's per-op cost
// (no fault layer) — the reference for the framework-tax delta below.
func BenchmarkExecutor_Baseline(b *testing.B) {
	exec := benchExec(b)
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_ = exec.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), noopFn)
	}
}

// BenchmarkFaultExecutor_Inert measures fault-wrapped overhead when the config
// is inert (no rate, no latency) — the steady-state cost a production config
// pays.
func BenchmarkFaultExecutor_Inert(b *testing.B) {
	exec := faultWrapped(benchExec(b), kfault.Config{})
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_ = exec.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), noopFn)
	}
}

// BenchmarkFaultExecutor_Injecting measures cost when every call is injected
// (Rate 1) — exercises the decision + error path per attempt.
func BenchmarkFaultExecutor_Injecting(b *testing.B) {
	conf := kfault.Config{Rules: []kfault.Rule{{
		Name: "bench", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic",
	}}}
	exec := faultWrapped(benchExec(b), conf)
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_ = exec.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), noopFn)
	}
}
