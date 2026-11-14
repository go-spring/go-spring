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

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/capability/source"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

const doc = `
services[0].service=redis:cache
services[0].rules[0].action.exact=-
services[0].rules[0].timeout=100ms
services[0].rules[0].retry.max-retries=2

services[0].rules[1].action.prefix=GET /slow
services[0].rules[1].match.tenant.exact=core
services[0].rules[1].timeout=2s
`

func TestDocumentBinds(t *testing.T) {
	d, err := source.Parse[resilience.TimeoutRetrySettings]("timeoutretry.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	assert.Number(t, len(d.Services)).Equal(1)
	assert.Number(t, len(d.Services[0].Rules)).Equal(2)
	assert.That(t, d.Services[0].Rules[0].ActionMatch.Exact).Equal("-")
	assert.That(t, d.Services[0].Rules[0].Timeout).Equal(100 * time.Millisecond)
	assert.Number(t, d.Services[0].Rules[0].Retry.MaxRetries).Equal(2)
	assert.That(t, d.Services[0].Rules[1].ActionMatch.Prefix).Equal("GET /slow")
	assert.That(t, d.Services[0].Rules[1].Timeout).Equal(2 * time.Second)
}

// TestDocumentScopeRules pins the binding boundary: conf.Bind runs the
// data-plane Validate, so the label-scoped retry budget is rejected on an
// interface rule where the document is read. The rule matrix itself lives in
// resilience's own tests.
func TestDocumentScopeRules(t *testing.T) {
	_, err := source.Parse[resilience.TimeoutRetrySettings]("timeoutretry.properties", []byte(`
services[0].service=redis:cache
services[0].rules[0].action.prefix=GET /
services[0].rules[0].retry.retry-budget=4
`), "")
	assert.Error(t, err).NotNil()
}

func TestCenterAppliesAndKeepsLastGood(t *testing.T) {
	d, err := source.Parse[resilience.TimeoutRetrySettings]("timeoutretry.properties", []byte(doc), "")
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

	// The service label resolves the service-level rule. The policy is observed
	// the way a client sees it: through a call's record.
	rec := new(chain.Recorder)
	assert.Error(t, mgr.Wrap("redis:cache", nil).Execute(
		context.Background(), chain.Operation{Service: "redis:cache"}, rec,
		func(context.Context, *chain.Round) error { return nil })).Nil()
	assert.That(t, rec.LastRound().Timeout).Equal(100 * time.Millisecond)

	// The interface rule wins for a matching call.
	rec = new(chain.Recorder)
	assert.Error(t, mgr.Wrap("redis:cache", nil).Execute(
		context.Background(), chain.Operation{
			Service: "redis:cache",
			Request: chain.Request{Action: "GET /slow/x", Metadata: map[string]string{"tenant": "core"}},
		}, rec, func(context.Context, *chain.Round) error { return nil })).Nil()
	assert.That(t, rec.LastRound().Timeout).Equal(2 * time.Second)
}
