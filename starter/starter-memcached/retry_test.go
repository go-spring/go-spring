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

package StarterMemcached

import (
	"context"
	"errors"
	"net"
	"testing"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// TestTransientErrorClassification pins the transport class: network errors are
// retry-eligible; server replies and ordinary errors are not.
func TestTransientErrorClassification(t *testing.T) {
	assert.That(t, isTransientError(&net.OpError{Op: "read", Net: "tcp", Err: errors.New("reset")})).True()
	assert.That(t, isTransientError(errors.New("NOT_STORED"))).False()
	assert.That(t, isTransientError(nil)).False()
}

// TestGuardRetriesTransient proves the transport class reaches the retry loop
// while the caller still receives the original error.
func TestGuardRetriesTransient(t *testing.T) {
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	m, err := resilience.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	assert.Error(t, m.ApplyTimeoutRetry(resilience.TimeoutRetrySettings{Services: []resilience.TimeoutRetryService{{
		Service: "svc",
		Rules: []resilience.TimeoutRetryRule{{
			ActionMatch:  matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			TimeoutRetry: resilience.TimeoutRetry{Retry: resilience.Retry{MaxRetries: 2}},
		}},
	}}})).Nil()

	boom := &net.OpError{Op: "read", Net: "tcp", Err: errors.New("reset")}
	var calls int
	_, err = guard(context.Background(), m.Wrap("svc", nil), chain.Operation{Service: "svc"},
		func(context.Context, chain.Peer) (struct{}, error) {
			calls++
			return struct{}{}, boom
		})
	assert.Error(t, err).Is(boom)
	assert.Number(t, calls).Equal(3)
}
