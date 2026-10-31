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

	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

const doc = `
rules[0].service=redis:cache
rules[0].action.prefix=GET /slow
rules[0].rate-limit=0.001
rules[0].burst=1
`

func TestDocumentBinds(t *testing.T) {
	d, err := source.Parse[resilience.RateLimitSettings]("rate-limit.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	assert.Number(t, len(d.Rules)).Equal(1)
	assert.That(t, d.Rules[0].Action.Prefix).Equal("GET /slow")
	assert.Number(t, d.Rules[0].Burst).Equal(1)
}

// TestDocumentRequiresInterfaceScope pins the binding boundary: conf.Bind runs
// the data-plane Validate, so the service marker is rejected where the document
// is read. The rule matrix itself lives in resilience's own tests.
func TestDocumentRequiresInterfaceScope(t *testing.T) {
	_, err := source.Parse[resilience.RateLimitSettings]("rate-limit.properties", []byte(`
rules[0].service=redis:cache
rules[0].action.exact=-
rules[0].rate-limit=10
`), "")
	assert.Error(t, err).NotNil()
}

func TestCenterAppliesAndKeepsLastGood(t *testing.T) {
	d, err := source.Parse[resilience.RateLimitSettings]("rate-limit.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	mgr := resilience.NewManager(resilience.NewDefaultDriver(nil))
	src := source.NewPushSource(d)
	if _, err := NewCenter(context.Background(), mgr, src); err != nil {
		t.Fatal(err)
	}

	e := mgr.ExecutorFor("redis:cache")
	noop := func(context.Context) error { return nil }
	slow := chain.Operation{Request: chain.Request{Action: "GET /slow/x"}}
	other := chain.Operation{Request: chain.Request{Action: "GET /fast"}}

	assert.Error(t, e.Execute(context.Background(), slow, nil, noop)).Nil()
	assert.Error(t, e.Execute(context.Background(), slow, nil, noop)).Is(chain.ErrRateLimited)
	// Another interface is not limited.
	for range 3 {
		assert.Error(t, e.Execute(context.Background(), other, nil, noop)).Nil()
	}

	// A rejected document keeps the exhausted budget serving.
	src.Push(context.Background(), resilience.RateLimitSettings{Rules: []resilience.RateLimitRule{{Service: "redis:cache"}}})
	assert.Error(t, e.Execute(context.Background(), slow, nil, noop)).Is(chain.ErrRateLimited)
}

// TestNewCenterRejectsUnservableSnapshot pins the startup contract: a source
// whose initial document cannot serve fails wiring, rather than arming a center
// that would run unconfigured.
func TestNewCenterRejectsUnservableSnapshot(t *testing.T) {
	mgr := resilience.NewManager(resilience.NewDefaultDriver(nil))
	src := source.NewPushSource(resilience.RateLimitSettings{
		Rules: []resilience.RateLimitRule{{Service: "redis:cache"}},
	})
	if _, err := NewCenter(context.Background(), mgr, src); err == nil {
		t.Fatal("NewCenter must reject an unservable initial document")
	}
}
