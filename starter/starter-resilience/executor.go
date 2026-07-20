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

package StarterResilience

import (
	"context"
	"fmt"
	"sync"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/isolation"

	"go-spring.org/spring/resilience"
)

// sentinelExecutor maps a backend-neutral resilience.Policy onto sentinel-golang
// rules. sentinel keys everything by resource name, so rules are loaded lazily
// the first time a resource is seen; retry and per-attempt timeout are applied
// around sentinel's entry check, since sentinel itself models neither.
type sentinelExecutor struct {
	policy resilience.Policy

	mu     sync.Mutex
	loaded map[string]bool
}

func newSentinelExecutor(p resilience.Policy) (resilience.Executor, error) {
	if p.RateLimit < 0 {
		return nil, fmt.Errorf("resilience: negative rate limit %v", p.RateLimit)
	}
	return &sentinelExecutor{policy: p, loaded: map[string]bool{}}, nil
}

// ensureRules loads flow and circuit-breaker rules for resource once, translating
// the neutral Policy knobs into sentinel's own rule shapes.
func (e *sentinelExecutor) ensureRules(resource string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loaded[resource] {
		return nil
	}

	if e.policy.RateLimit > 0 {
		if _, err := flow.LoadRulesOfResource(resource, []*flow.Rule{{
			Resource:               resource,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			Threshold:              e.policy.RateLimit,
			StatIntervalInMs:       1000,
		}}); err != nil {
			return fmt.Errorf("resilience: load flow rule for %q: %w", resource, err)
		}
	}

	if e.policy.ErrorThreshold > 0 {
		openMs := uint32(e.policy.OpenDuration.Milliseconds())
		if openMs == 0 {
			openMs = 5000
		}
		if _, err := circuitbreaker.LoadRulesOfResource(resource, []*circuitbreaker.Rule{{
			Resource:         resource,
			Strategy:         circuitbreaker.ErrorCount,
			RetryTimeoutMs:   openMs,
			MinRequestAmount: 1,
			StatIntervalMs:   1000,
			Threshold:        float64(e.policy.ErrorThreshold),
		}}); err != nil {
			return fmt.Errorf("resilience: load breaker rule for %q: %w", resource, err)
		}
	}

	if e.policy.MaxConcurrent > 0 {
		// sentinel's isolation slot tracks in-flight concurrency per resource via
		// the Entry/Exit pair below, so the neutral bulkhead maps straight onto a
		// concurrency rule with no extra bookkeeping on our side.
		if _, err := isolation.LoadRulesOfResource(resource, []*isolation.Rule{{
			Resource:   resource,
			MetricType: isolation.Concurrency,
			Threshold:  uint32(e.policy.MaxConcurrent),
		}}); err != nil {
			return fmt.Errorf("resilience: load isolation rule for %q: %w", resource, err)
		}
	}

	e.loaded[resource] = true
	return nil
}

func (e *sentinelExecutor) Execute(ctx context.Context, resource string, fn func(context.Context) error) error {
	if err := e.ensureRules(resource); err != nil {
		return err
	}

	attempts := e.policy.MaxRetries + 1
	var err error
	for range attempts {
		entry, blockErr := sentinel.Entry(resource, sentinel.WithTrafficType(base.Outbound))
		if blockErr != nil {
			return mapBlockError(blockErr)
		}

		err = e.runOnce(ctx, fn)
		if err != nil {
			sentinel.TraceError(entry, err)
		}
		entry.Exit()

		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return err
}

// runOnce applies the per-attempt timeout, if any, around fn.
func (e *sentinelExecutor) runOnce(ctx context.Context, fn func(context.Context) error) error {
	if e.policy.Timeout <= 0 {
		return fn(ctx)
	}
	attemptCtx, cancel := context.WithTimeout(ctx, e.policy.Timeout)
	defer cancel()
	return fn(attemptCtx)
}

func (e *sentinelExecutor) Close() error { return nil }

// mapBlockError translates sentinel's block reason into the framework's neutral
// sentinel errors so callers depend only on go-spring.org/spring/resilience.
func mapBlockError(b *base.BlockError) error {
	switch b.BlockType() {
	case base.BlockTypeCircuitBreaking:
		return fmt.Errorf("%w: %s", resilience.ErrCircuitOpen, b.Error())
	case base.BlockTypeIsolation:
		return fmt.Errorf("%w: %s", resilience.ErrBulkheadFull, b.Error())
	default:
		return fmt.Errorf("%w: %s", resilience.ErrRateLimited, b.Error())
	}
}
