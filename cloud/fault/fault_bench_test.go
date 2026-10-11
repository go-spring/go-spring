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

	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/resilience"
)

// noopFn is the operation handed to the executor: returns nil immediately so
// the benchmark isolates framework overhead (executor + injector path), not any
// downstream work.
func noopFn(context.Context) error { return nil }

// BenchmarkExecutor_Baseline measures the raw default executor's per-op cost
// (no fault layer) — the reference for the framework-tax delta below.
func BenchmarkExecutor_Baseline(b *testing.B) {
	d := resilience.NewDefaultDriver(nil)
	exec, _ := d.NewClientExecutor("svc", resilience.ClientPolicy{})
	defer func() { _ = exec.Close() }()
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_ = exec.Execute(ctx, chain.Operation{}, noopFn)
	}
}

// BenchmarkFaultExecutor_Inert measures fault-wrapped overhead when the config
// is inert (no rate, no latency) — the steady-state cost a production config
// pays.
func BenchmarkFaultExecutor_Inert(b *testing.B) {
	d := resilience.NewDefaultDriver(nil)
	raw, _ := d.NewClientExecutor("svc", resilience.ClientPolicy{})
	defer func() { _ = raw.Close() }()
	exec := WrapClientExecutor(raw, "svc", NewInjector(Configs{Client: Config{}}, nil)) // inert
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_ = exec.Execute(ctx, chain.Operation{}, noopFn)
	}
}

// BenchmarkFaultExecutor_Injecting measures cost when every call is injected
// (Rate 1) — exercises the maybe() decision + error path per attempt.
func BenchmarkFaultExecutor_Injecting(b *testing.B) {
	d := resilience.NewDefaultDriver(nil)
	raw, _ := d.NewClientExecutor("svc", resilience.ClientPolicy{})
	defer func() { _ = raw.Close() }()
	exec := WrapClientExecutor(raw, "svc", NewInjector(Configs{Client: Config{Rate: 1, Error: "generic"}}, nil))
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_ = exec.Execute(ctx, chain.Operation{}, noopFn)
	}
}
