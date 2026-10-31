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

package circuitbreaker

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

const doc = `
rules[0].service=redis:cache
rules[0].action.exact=-
rules[0].error-threshold=1
rules[0].open-duration=1m

rules[1].service=redis:cache
rules[1].action.prefix=GET /
rules[1].error-threshold=100
`

func TestDocumentBinds(t *testing.T) {
	d, err := source.Parse[resilience.BreakerSettings]("breaker.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	assert.Number(t, len(d.Rules)).Equal(2)
	assert.That(t, d.Rules[0].Action.Exact).Equal("-")
	assert.Number(t, d.Rules[0].ErrorThreshold).Equal(1)
	assert.That(t, d.Rules[1].Action.Prefix).Equal("GET /")
	assert.Number(t, d.Rules[1].ErrorThreshold).Equal(100)
}

// TestDocumentRejectsInvalidRules pins the binding boundary: conf.Bind runs the
// data-plane Validate, so a rule the breaker cannot build is rejected where the
// document is read. The rule matrix itself lives in resilience's own tests.
func TestDocumentRejectsInvalidRules(t *testing.T) {
	_, err := source.Parse[resilience.BreakerSettings]("breaker.properties", []byte(`
rules[0].service=svc
rules[0].action.exact=-
rules[0].breaker-strategy=slow-call
`), "")
	assert.Error(t, err).NotNil()
}

func TestServiceRuleWinsOutright(t *testing.T) {
	d, err := source.Parse[resilience.BreakerSettings]("breaker.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	mgr := resilience.NewManager(resilience.NewDefaultDriver(nil))
	src := source.NewPushSource(d)
	if _, err := NewCenter(context.Background(), mgr, src); err != nil {
		t.Fatal(err)
	}

	e := mgr.ExecutorFor("redis:cache")
	boom := errors.New("boom")
	fail := func() error {
		return e.Execute(context.Background(), chain.Operation{
			Request: chain.Request{Action: "GET /x"},
		}, nil, func(context.Context) error { return boom })
	}

	// The service rule (threshold 1) governs even though an interface rule with
	// threshold 100 also matches: one failure trips the service breaker.
	assert.Error(t, fail()).Is(boom)
	assert.Error(t, e.Execute(context.Background(), chain.Operation{
		Request: chain.Request{Action: "GET /x"},
	}, nil, func(context.Context) error { return nil })).Is(chain.ErrCircuitOpen)

	// A rejected document keeps the tripped breaker's service policy servicing.
	src.Push(context.Background(), resilience.BreakerSettings{Rules: []resilience.BreakerRule{{Service: "redis:cache"}}})
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil })).Is(chain.ErrCircuitOpen)
}

// TestNewCenterRejectsUnservableSnapshot pins the startup contract: a source
// whose initial document cannot serve fails wiring, rather than arming a center
// that would run unconfigured.
func TestNewCenterRejectsUnservableSnapshot(t *testing.T) {
	mgr := resilience.NewManager(resilience.NewDefaultDriver(nil))
	src := source.NewPushSource(resilience.BreakerSettings{
		Rules: []resilience.BreakerRule{{Service: ""}},
	})
	if _, err := NewCenter(context.Background(), mgr, src); err == nil {
		t.Fatal("NewCenter must reject an unservable initial document")
	}
}
