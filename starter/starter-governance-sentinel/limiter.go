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

// limiter.go contributes sentinel-golang as a RATE-LIMIT ALGORITHM, not as a
// second executor. It implements [ratelimit.Provider], so a policy selects it
// exactly like the bundled token bucket (`limiter: sentinel`) while the
// resilience pipeline keeps ownership of timeout, retry, rule matching,
// per-rule keys and metrics.
package StarterGovernanceSentinel

import (
	"context"
	"sync"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/flow"

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/stdlib/errutil"
)

// LimiterAlgorithm is the algorithm name a rule writes to run its rate limit on
// sentinel's flow control: `limiter: sentinel`.
const LimiterAlgorithm = "sentinel"

// Provider exports the sentinel-backed limiter algorithm to the process's
// [ratelimit.Factory]. Contribute it as a [ratelimit.Provider] bean (the
// starter does this on import); the factory rejects a rule naming an algorithm
// no provider exports.
type Provider struct{}

var _ ratelimit.Provider = Provider{}

// Algorithms reports the one algorithm this provider answers.
func (Provider) Algorithms() []string { return []string{LimiterAlgorithm} }

// New binds a flow counter to scope — the rule's identity key, so two rules of
// one service get independent budgets. The counter holds no state of its own:
// sentinel keys everything by resource name.
func (Provider) New(algorithm, scope string) (ratelimit.Limiter, error) {
	if algorithm != LimiterAlgorithm {
		return nil, errutil.Explain(nil, "sentinel: unsupported rate-limit algorithm %q", algorithm)
	}
	return &flowLimiter{resource: scope}, nil
}

// flowLimiter charges one scope's budget in sentinel. The neutral policy
// travels with every call, so a rule change is a rule swap on the resource, not
// a new counter.
type flowLimiter struct {
	resource string

	mu     sync.Mutex
	policy ratelimit.Policy
	loaded bool
}

func (l *flowLimiter) Allow(_ context.Context, p ratelimit.Policy, n int) (bool, error) {
	if p.RateLimit <= 0 || n <= 0 {
		l.clear()
		return true, nil
	}
	if err := l.load(p); err != nil {
		return false, err
	}
	entry, blockErr := sentinel.Entry(l.resource,
		sentinel.WithBatchCount(uint32(n)),
		sentinel.WithTrafficType(base.Outbound))
	if blockErr != nil {
		// Budget exhausted (or queued out): a verdict, not a store failure.
		return false, nil
	}
	entry.Exit()
	return true, nil
}

func (l *flowLimiter) load(p ratelimit.Policy) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.loaded && l.policy == p {
		return nil
	}
	rule := flowRule(l.resource, flowKnobs{
		rateLimit:  p.RateLimit,
		window:     p.Window,
		maxWait:    p.MaxWait,
		warmUp:     p.WarmUp,
		coldFactor: p.ColdFactor,
	})
	if _, err := flow.LoadRulesOfResource(l.resource, []*flow.Rule{rule}); err != nil {
		return errutil.Explain(err, "sentinel: flow rule for %q", l.resource)
	}
	l.policy, l.loaded = p, true
	return nil
}

func (l *flowLimiter) clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.loaded {
		return
	}
	_, _ = flow.LoadRulesOfResource(l.resource, nil)
	l.policy, l.loaded = ratelimit.Policy{}, false
}
