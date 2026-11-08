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

package admission

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/capability/source"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	"go-spring.org/stdlib/testing/assert"
)

const doc = `
rate-limit=100
services[0].service=gin::8080
services[0].max-concurrent=50
`

func TestDocumentBinds(t *testing.T) {
	d, err := source.Parse[admission.Settings]("admission.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	assert.Number(t, d.Default.RateLimit).Equal(100)
	assert.That(t, d.Default.Limiter).Equal(ratelimit.TokenBucketAlgorithm)
	assert.Number(t, len(d.Rules)).Equal(1)
	assert.That(t, d.Rules[0].Service).Equal("gin::8080")
	assert.Number(t, d.Rules[0].MaxConcurrent).Equal(50)
	assert.That(t, d.Rules[0].Limiter).Equal(ratelimit.TokenBucketAlgorithm)
}

// TestDuplicateEntrancesAreOrderShadowed pins that a repeated entrance is not a
// document error: matching takes the first one, and avoiding duplicates is the
// platform's job when it writes the document.
func TestDuplicateEntrancesAreOrderShadowed(t *testing.T) {
	_, err := source.Parse[admission.Settings]("admission.properties", []byte(`
services[0].service=gin::8080
services[1].service=gin::8080
`), "")
	assert.Error(t, err).Nil()
}

func TestCenterAppliesAndKeepsLastGood(t *testing.T) {
	d, err := source.Parse[admission.Settings]("admission.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := admission.NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	// A guard must exist for Apply to materialize a policy: construction is what
	// rejects a document, so nothing is built for an instance this process does
	// not serve.
	if _, err := mgr.GuardFor(admission.Identity{System: "gin", Service: "gin::8080"}); err != nil {
		t.Fatal(err)
	}
	src := source.NewPushSource(d)
	if _, err := NewCenter(context.Background(), mgr, src); err != nil {
		t.Fatal(err)
	}

	assert.Number(t, mgr.PolicyFor("gin::8080").MaxConcurrent).Equal(50)
	assert.Number(t, mgr.PolicyFor("echo::9090").RateLimit).Equal(100)

	// A policy that cannot be built is rejected; the previous one keeps serving.
	src.Push(context.Background(), admission.Settings{Rules: []admission.Rule{
		{Service: "gin::8080", Policy: admission.Policy{MaxConcurrent: -1, Timeout: time.Second}},
	}})
	assert.Number(t, mgr.PolicyFor("gin::8080").MaxConcurrent).Equal(50)
}

// TestNewCenterRejectsUnservableSnapshot pins the startup contract: a source
// whose initial document cannot be built fails wiring, rather than arming a
// center that would run unconfigured.
func TestNewCenterRejectsUnservableSnapshot(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := admission.NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.GuardFor(admission.Identity{System: "gin", Service: "gin::8080"}); err != nil {
		t.Fatal(err)
	}
	src := source.NewPushSource(admission.Settings{
		Rules: []admission.Rule{{Service: "gin::8080", Policy: admission.Policy{MaxConcurrent: -1}}},
	})
	if _, err := NewCenter(context.Background(), mgr, src); err == nil {
		t.Fatal("NewCenter must reject an unservable initial document")
	}
}
