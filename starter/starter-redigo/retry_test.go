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

package StarterRedigo

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
	assert.That(t, isTransientError(errors.New("WRONGTYPE"))).False()
	assert.That(t, isTransientError(nil)).False()
}

// TestInterceptorRetriesTransient proves the transport class reaches the retry
// loop while the caller still receives the original error.
func TestInterceptorRetriesTransient(t *testing.T) {
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

	interceptor := resilienceInterceptor(m.Wrap("svc", nil), "svc", testProp)
	boom := &net.OpError{Op: "read", Net: "tcp", Err: errors.New("reset")}
	var calls int
	handler := interceptor(func(_ context.Context, _ chain.Peer, _ string, _ []interface{}) (interface{}, error) {
		calls++
		return nil, boom
	})
	_, seen := handler(context.Background(), chain.Peer{}, "GET", []interface{}{"k"})
	assert.Error(t, seen).Is(boom)
	assert.Number(t, calls).Equal(3)
}
