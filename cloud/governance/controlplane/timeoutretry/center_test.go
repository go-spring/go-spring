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

package timeoutretry

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

const doc = `
rules[0].service=redis:cache
rules[0].action.exact=-
rules[0].timeout=100ms
rules[0].retry.max-retries=2

rules[1].service=redis:cache
rules[1].action.prefix=GET /slow
rules[1].match.tenant=core
rules[1].timeout=2s
`

func TestDocumentBinds(t *testing.T) {
	d, err := source.Parse[resilience.LimitsSettings]("timeoutretry.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	assert.Number(t, len(d.Rules)).Equal(2)
	assert.That(t, d.Rules[0].Action.Exact).Equal("-")
	assert.That(t, d.Rules[0].Timeout).Equal(100 * time.Millisecond)
	assert.Number(t, d.Rules[0].Retry.MaxRetries).Equal(2)
	assert.That(t, d.Rules[1].Action.Prefix).Equal("GET /slow")
	assert.That(t, d.Rules[1].Timeout).Equal(2 * time.Second)
}

// TestDocumentScopeRules pins the binding boundary: conf.Bind runs the
// data-plane Validate, so the label-scoped retry budget is rejected on an
// interface rule where the document is read. The rule matrix itself lives in
// resilience's own tests.
func TestDocumentScopeRules(t *testing.T) {
	_, err := source.Parse[resilience.LimitsSettings]("timeoutretry.properties", []byte(`
rules[0].service=redis:cache
rules[0].action.prefix=GET /
rules[0].retry.retry-budget=4
`), "")
	assert.Error(t, err).NotNil()
}

func TestCenterAppliesAndKeepsLastGood(t *testing.T) {
	d, err := source.Parse[resilience.LimitsSettings]("timeoutretry.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	mgr := resilience.NewManager(resilience.NewDefaultDriver(nil))
	src := source.NewPushSource(d)
	if _, err := NewCenter(context.Background(), mgr, src); err != nil {
		t.Fatal(err)
	}

	// The service label resolves the service-level rule.
	assert.That(t, mgr.LimitsFor("redis:cache").Timeout).Equal(100 * time.Millisecond)

	// The interface rule wins for a matching call.
	got := mgr.Resolve(chain.Operation{
		Service: "redis:cache",
		Request: chain.Request{Action: "GET /slow/x", Metadata: map[string]string{"tenant": "core"}},
	}).Limits.Timeout
	assert.That(t, got).Equal(2 * time.Second)

	// A rejected document keeps the last good one serving.
	src.Push(context.Background(), resilience.LimitsSettings{Rules: []resilience.LimitsRule{{Service: "redis:cache"}}})
	assert.That(t, mgr.LimitsFor("redis:cache").Timeout).Equal(100 * time.Millisecond)
}

// TestNewCenterRejectsUnservableSnapshot pins the startup contract: a source
// whose initial document cannot serve fails wiring, rather than arming a center
// that would run unconfigured.
func TestNewCenterRejectsUnservableSnapshot(t *testing.T) {
	mgr := resilience.NewManager(resilience.NewDefaultDriver(nil))
	src := source.NewPushSource(resilience.LimitsSettings{
		Rules: []resilience.LimitsRule{{Service: "redis:cache"}},
	})
	if _, err := NewCenter(context.Background(), mgr, src); err == nil {
		t.Fatal("NewCenter must reject an unservable initial document")
	}
}
