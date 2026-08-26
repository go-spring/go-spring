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

package loadtest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// TestIntegration_LoadFaultBreaker is the capstone: it proves the load-test +
// fault-injection + resilience stack actually engage together — the whole
// reason this trio exists. A resilience executor with a sensitive breaker wraps
// an op that would succeed, but a fault injector makes it fail at rate 1. Under
// load the breaker must open, the load harness must classify those rejects into
// the Circuit bucket, and the registered assertion must pass a verdict.
//
// This is the closed loop the design calls "证明保护机制不是摆设": 放火制造故障,
// 压测制造压力, 断言熔断真的开。
func TestIntegration_LoadFaultBreaker(t *testing.T) {
	d, err := resilience.GetDriver("default")
	assert.Error(t, err).Nil()
	exec, err := d.NewExecutor(resilience.Policy{ErrorThreshold: 3, OpenDuration: time.Minute})
	assert.Error(t, err).Nil()
	t.Cleanup(func() { _ = exec.Close() })

	// Fault wraps the executor: every call returns an injected error.
	fexec := fault.WrapExecutorWith(exec, fault.NewInjector(fault.Config{
		Enabled: true, Rate: 1, Error: "generic",
	}))

	// The op routes through the faulted executor; its inner fn would succeed,
	// so any failure is fault/resilience, not the op itself.
	op := func(ctx context.Context) error {
		return fexec.Execute(ctx, "svc", func(context.Context) error { return nil })
	}

	r := New().
		Driver(ClosedLoop{Concurrency: 8}).
		Duration(200*time.Millisecond).
		Assert("breaker-opened", func(_ context.Context, r *Result) error {
			if r.Buckets[BucketCircuit] == 0 {
				return fmt.Errorf("breaker never opened under fault (circuit=%d)", r.Buckets[BucketCircuit])
			}
			return nil
		}).
		Assert("faults-counted", func(_ context.Context, r *Result) error {
			// Injected faults drove the breaker open; the injected bucket must be
			// non-zero (the failures that tripped it before it opened).
			if r.Buckets[BucketInjected] == 0 {
				return fmt.Errorf("no injected faults recorded")
			}
			return nil
		}).
		Run(context.Background(), op)

	assert.That(t, r.Passed()).True()                    // both assertions held
	assert.That(t, r.Buckets[BucketCircuit] > 0).True()  // breaker opened, requests rejected as circuit-open
	assert.That(t, r.Buckets[BucketInjected] > 0).True() // the injected failures that tripped it
}

// TestIntegration_ScopeRealTrafficSkipsFault proves fault.Scope gates the
// closed loop the other way: with scope "real", load-test traffic (which the
// harness tags) is NOT faulted, so the breaker never opens.
func TestIntegration_ScopeRealTrafficSkipsFault(t *testing.T) {
	d, err := resilience.GetDriver("default")
	assert.Error(t, err).Nil()
	exec, err := d.NewExecutor(resilience.Policy{ErrorThreshold: 1, OpenDuration: time.Minute})
	assert.Error(t, err).Nil()
	t.Cleanup(func() { _ = exec.Close() })

	fexec := fault.WrapExecutorWith(exec, fault.NewInjector(fault.Config{
		Enabled: true, Rate: 1, Error: "generic", Scope: "real", // skip load-test traffic
	}))
	op := func(ctx context.Context) error {
		return fexec.Execute(ctx, "svc", func(context.Context) error { return nil })
	}

	r := New().Driver(ClosedLoop{Concurrency: 4}).Duration(150*time.Millisecond).
		Run(context.Background(), op)
	// Load-test traffic is excluded by scope "real": no faults, no breaker trip.
	assert.That(t, r.Buckets[BucketInjected]).Equal(int64(0))
	assert.That(t, r.Buckets[BucketCircuit]).Equal(int64(0))
	assert.That(t, r.Errors()).Equal(int64(0))
}
