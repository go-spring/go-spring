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

package ratelimit

import (
	"context"
	"testing"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/capability/source"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

const doc = `
services[0].service=redis:cache
services[0].rules[0].action.prefix=GET /slow
services[0].rules[0].rate-limit=0.001
services[0].rules[0].burst=1
`

func TestDocumentBinds(t *testing.T) {
	d, err := source.Parse[resilience.RateLimitSettings]("rate-limit.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	assert.Number(t, len(d.Services)).Equal(1)
	assert.Number(t, len(d.Services[0].Rules)).Equal(1)
	assert.That(t, d.Services[0].Rules[0].ActionMatch.Prefix).Equal("GET /slow")
	assert.Number(t, d.Services[0].Rules[0].Burst).Equal(1)
	// An omitted limiter binds to the documented default, so no downstream
	// consumer ever sees an empty kind.
	assert.That(t, d.Services[0].Rules[0].Limiter).Equal(ratelimit.TokenBucketAlgorithm)
}

// TestDocumentAcceptsServiceScope pins the binding boundary: conf.Bind runs the
// data-plane Validate, and the service marker is a valid scope — a whole-service
// fallback. The rule matrix itself lives in resilience's own tests.
func TestDocumentAcceptsServiceScope(t *testing.T) {
	d, err := source.Parse[resilience.RateLimitSettings]("rate-limit.properties", []byte(`
services[0].service=redis:cache
services[0].rules[0].action.exact=-
services[0].rules[0].rate-limit=10
`), "")
	assert.Error(t, err).Nil()
	assert.That(t, d.Services[0].Rules[0].ActionMatch.Exact).Equal(matcher.ServiceScopeAction)
}

func TestCenterAppliesAndKeepsLastGood(t *testing.T) {
	d, err := source.Parse[resilience.RateLimitSettings]("rate-limit.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := resilience.NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	src := source.NewPushSource(d)
	if _, err := NewCenter(context.Background(), mgr, src); err != nil {
		t.Fatal(err)
	}

	e := mgr.Wrap("redis:cache", nil)
	noop := func(context.Context) error { return nil }
	slow := chain.Operation{Service: "redis:cache", Request: chain.Request{Action: "GET /slow/x"}}
	other := chain.Operation{Service: "redis:cache", Request: chain.Request{Action: "GET /fast"}}

	assert.Error(t, e.Execute(context.Background(), slow, new(chain.Recorder), noop)).Nil()
	assert.Error(t, e.Execute(context.Background(), slow, new(chain.Recorder), noop)).Is(chain.ErrRateLimited)
	// Another interface is not limited.
	for range 3 {
		assert.Error(t, e.Execute(context.Background(), other, new(chain.Recorder), noop)).Nil()
	}

	// A document whose rule names a kind no provider exports cannot be built, so
	// the previous one keeps the exhausted budget serving.
	src.Push(context.Background(), resilience.RateLimitSettings{Services: []resilience.RateLimitService{{
		Service: "redis:cache",
		Rules: []resilience.RateLimitRule{{
			Policy: ratelimit.Policy{RateLimit: 1, Limiter: "redis:unregistered"},
		}},
	}}})
	assert.Error(t, e.Execute(context.Background(), slow, new(chain.Recorder), noop)).Is(chain.ErrRateLimited)
}

// TestNewCenterRejectsUnservableSnapshot pins the startup contract: a source
// whose initial document cannot be built fails wiring, rather than arming a
// center that would run unconfigured.
func TestNewCenterRejectsUnservableSnapshot(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := resilience.NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	src := source.NewPushSource(resilience.RateLimitSettings{
		Services: []resilience.RateLimitService{{
			Service: "redis:cache",
			Rules: []resilience.RateLimitRule{{
				Policy: ratelimit.Policy{RateLimit: 1, Limiter: "redis:unregistered"},
			}},
		}},
	})
	if _, err := NewCenter(context.Background(), mgr, src); err == nil {
		t.Fatal("NewCenter must reject an unservable initial document")
	}
}
