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

package resilience

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
)

// TestConnectTimeoutStagedOnRound pins the budget's travel: a rule that states
// connect-timeout puts the value on every round's context, where a dial site
// beneath the executor reads it; a rule that states none stages nothing.
func TestConnectTimeoutStagedOnRound(t *testing.T) {
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.That(t, err).Nil()
	mgr, err := NewManager(fact)
	assert.That(t, err).Nil()
	defer func() { _ = mgr.Close() }()
	assert.That(t, mgr.ApplyTimeoutRetry(TimeoutRetrySettings{Services: []TimeoutRetryService{{
		Service: "svc",
		Rules: []TimeoutRetryRule{{
			ActionMatch:  matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			TimeoutRetry: TimeoutRetry{Timeout: 10 * time.Second, ConnectTimeout: 300 * time.Millisecond},
		}},
	}}})).Nil()

	exec := mgr.Wrap("svc", nil)

	saw, staged := 0, 0
	err = exec.Execute(context.Background(), chain.Operation{Service: "svc"}, new(chain.Recorder),
		func(ctx context.Context, _ *chain.Round) error {
			saw++
			if rd, ok := chain.RoundFrom(ctx); ok && rd.ConnectTimeout > 0 {
				staged++
				assert.That(t, rd.ConnectTimeout).Equal(300 * time.Millisecond)
			}
			return nil
		})
	assert.That(t, err).Nil()
	assert.That(t, saw > 0).True()
	assert.That(t, staged).Equal(saw)
}
