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

package StarterGovernanceSentinel

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
	"go-spring.org/cloud/runtime/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// The equivalence tests pin that the two drivers answer the SAME sequence of
// calls with the SAME accept/reject verdicts for the stages both implement.
// They are the executable half of "the neutral policy means the same thing
// whichever engine runs it": where the engines legitimately differ (warm-up's
// ramp shape, windowed vs run counting), the dedicated per-driver tests own it.

// equivCase is one deterministic scenario: a policy and a script of calls whose
// outcomes the drivers must agree on, verdict by verdict.
type equivCase struct {
	name   string
	policy resilience.Policy
	script func(e chain.Executor) []error
}

// runEquiv drives one case through both drivers and compares verdicts: for
// each call, whether it was accepted (nil) and, when rejected, which neutral
// sentinel error it carried.
func runEquiv(t *testing.T, c equivCase) {
	t.Helper()
	def, err := resilience.NewDefaultDriver(nil).NewExecutor("equiv:"+c.name, c.policy)
	assert.Error(t, err).Nil()
	sen, err := NewSentinelDriver().NewExecutor("equiv-sentinel:"+c.name, c.policy)
	assert.Error(t, err).Nil()

	want := c.script(def)
	got := c.script(sen)
	assert.That(t, len(got)).Equal(len(want))
	for i := range want {
		verdict := func(err error) string {
			if err == nil {
				return "ok"
			}
			for _, s := range []error{chain.ErrRateLimited, chain.ErrCircuitOpen, chain.ErrBulkheadFull} {
				if errors.Is(err, s) {
					return s.Error()
				}
			}
			return "fail"
		}
		assert.That(t, verdict(got[i])).Equal(verdict(want[i]),
			fmt.Sprintf("call %d: drivers disagree", i))
	}
}

func TestEquivRateLimit(t *testing.T) {
	runEquiv(t, equivCase{
		name:   "ratelimit",
		policy: resilience.Policy{RateLimit: 2},
		script: func(e chain.Executor) []error {
			var out []error
			for range 3 {
				out = append(out, e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil }))
			}
			return out
		},
	})
}

func TestEquivConsecutiveBreaker(t *testing.T) {
	runEquiv(t, equivCase{
		name:   "breaker",
		policy: resilience.Policy{ErrorThreshold: 1, OpenDuration: time.Minute},
		script: func(e chain.Executor) []error {
			var out []error
			// Trip: one failure.
			out = append(out, e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error {
				return errors.New("boom")
			}))
			// Open: the next call is short-circuited.
			out = append(out, e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil }))
			return out
		},
	})
}
