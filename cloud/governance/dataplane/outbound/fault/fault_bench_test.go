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
 * See the License for the specific language governing permissions.
 *
 */

package fault

import (
	"context"
	"testing"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
)

// noopFn is the operation handed to the executor: returns nil immediately so
// the benchmark isolates framework overhead (executor + injector path), not any
// downstream work.
func noopFn(context.Context) error { return nil }

// benchExec builds a default (policy-free) executor for "svc", released at the
// benchmark's cleanup.
func benchExec(b *testing.B) chain.Executor {
	b.Helper()
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		b.Fatal(err)
	}
	m := resilience.NewManager(resilience.NewDefaultDriver(), factories)
	b.Cleanup(func() { _ = m.Close() })
	return m.ExecutorFor("svc")
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
	raw := benchExec(b)
	exec := newFixture(configs{Client: kfault.Config{}}).client.WrapClientExecutor(raw) // inert
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_ = exec.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), noopFn)
	}
}

// BenchmarkFaultExecutor_Injecting measures cost when every call is injected
// (Rate 1) — exercises the maybe() decision + error path per attempt.
func BenchmarkFaultExecutor_Injecting(b *testing.B) {
	raw := benchExec(b)
	exec := newFixture(configs{Client: kfault.Config{Rules: []kfault.Rule{{Name: "bench", Service: "svc", Rate: 1, Class: kfault.ClassTransient, Error: "generic"}}}}).client.WrapClientExecutor(raw)
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_ = exec.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), noopFn)
	}
}
