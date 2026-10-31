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

	"go-spring.org/cloud/governance/controlplane/source"
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
	assert.Number(t, len(d.Rules)).Equal(1)
	assert.That(t, d.Rules[0].Service).Equal("gin::8080")
	assert.Number(t, d.Rules[0].MaxConcurrent).Equal(50)
}

func TestDuplicateEntrancesRejected(t *testing.T) {
	_, err := source.Parse[admission.Settings]("admission.properties", []byte(`
services[0].service=gin::8080
services[1].service=gin::8080
`), "")
	assert.Error(t, err).NotNil()
}

func TestCenterAppliesAndKeepsLastGood(t *testing.T) {
	d, err := source.Parse[admission.Settings]("admission.properties", []byte(doc), "")
	assert.Error(t, err).Nil()

	mgr := admission.NewManager()
	src := source.NewPushSource(d)
	if _, err := NewCenter(context.Background(), mgr, src); err != nil {
		t.Fatal(err)
	}

	assert.Number(t, mgr.PolicyFor("gin::8080").MaxConcurrent).Equal(50)
	assert.Number(t, mgr.PolicyFor("echo::9090").RateLimit).Equal(100)

	// A duplicate document is rejected; the previous one keeps serving.
	src.Push(context.Background(), admission.Settings{Rules: []admission.Rule{
		{Service: "gin::8080", Policy: admission.Policy{MaxConcurrent: 1, Timeout: time.Second}},
		{Service: "gin::8080", Policy: admission.Policy{MaxConcurrent: 2}},
	}})
	assert.Number(t, mgr.PolicyFor("gin::8080").MaxConcurrent).Equal(50)
}

// TestNewCenterRejectsUnservableSnapshot pins the startup contract: a source
// whose initial document cannot serve fails wiring, rather than arming a center
// that would run unconfigured.
func TestNewCenterRejectsUnservableSnapshot(t *testing.T) {
	mgr := admission.NewManager()
	src := source.NewPushSource(admission.Settings{
		Rules: []admission.Rule{{Service: ""}},
	})
	if _, err := NewCenter(context.Background(), mgr, src); err == nil {
		t.Fatal("NewCenter must reject an unservable initial document")
	}
}
