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

package fault

import (
	"context"
	"sync"
	"testing"
	"time"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/controlplane/source"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/stdlib/testing/assert"
)

// newTestCenter builds a center over src. A nil source stands for what the
// starter supplies when no source module is configured: an empty PushSource.
func newTestCenter(t *testing.T, src source.Source[kfault.Document]) *Center {
	t.Helper()
	if src == nil {
		src = source.NewPushSource(kfault.Document{})
	}
	c, err := NewCenter(context.Background(), clientfault.NewManager(), serverfault.NewManager(), src)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// doc is one valid document per direction, named after the rule it arms.
func doc(clientName, serverName string) kfault.Document {
	return kfault.Document{
		RunID:  "drill-test",
		Client: kfault.Config{Rules: []kfault.Rule{{Name: clientName, Rate: 1, Class: kfault.ClassTransient, Error: kfault.ErrorGeneric}}},
		Server: kfault.Config{Rules: []kfault.Rule{{Name: serverName, Rate: 1, Class: kfault.ClassTransient, Error: kfault.ErrorTimeout}}},
	}
}

// TestCenter_ArmsFromSnapshot pins the startup path: the source's initial
// snapshot is applied to both managers during construction.
func TestCenter_ArmsFromSnapshot(t *testing.T) {
	c := newTestCenter(t, source.NewPushSource(doc("client", "server")))

	cf, sf := c.client.Config(), c.server.Config()
	assert.Number(t, len(cf.Rules)).Equal(1)
	assert.String(t, cf.Rules[0].Name).Equal("client")
	assert.Number(t, len(sf.Rules)).Equal(1)
	assert.String(t, sf.Rules[0].Name).Equal("server")
}

// TestCenter_NoSourceStaysUnarmed: with no configured source the center arms on
// an empty document, and both directions inject nothing.
func TestCenter_NoSourceStaysUnarmed(t *testing.T) {
	c := newTestCenter(t, nil)
	assert.That(t, c.src).NotNil() // normalized to an empty document
	assert.Number(t, len(c.client.Config().Rules)).Equal(0)
	assert.Number(t, len(c.server.Config().Rules)).Equal(0)
}

// TestCenter_LatePushDrivesBothDirections: every push is applied to both
// directions, not just the one that changed.
func TestCenter_LatePushDrivesBothDirections(t *testing.T) {
	src := source.NewPushSource(kfault.Document{RunID: "drill-init"})
	c := newTestCenter(t, src)

	src.Push(context.Background(), doc("c2", "s2"))

	assert.String(t, c.client.Config().Rules[0].Name).Equal("c2")
	assert.String(t, c.server.Config().Rules[0].Name).Equal("s2")
}

// TestCenter_RunIDReachesBothDirections: one drill id in the document labels both
// directions, so an outbound and an inbound fire from the same push group under
// the same run.
func TestCenter_RunIDReachesBothDirections(t *testing.T) {
	c := newTestCenter(t, nil)
	d := doc("client", "server")
	d.RunID = "drill-7"
	assert.Error(t, c.Refresh(context.Background(), d)).Nil()
	assert.String(t, c.client.Config().RunID).Equal("drill-7")
	assert.String(t, c.server.Config().RunID).Equal("drill-7")
}

// TestCenter_DocumentRunIDOverridesDirections: Config.RunID has no binding tag,
// so a per-direction value can only come from code. The document-level id is
// authoritative and is applied BEFORE commit, so the two directions can never
// carry a different run than the document.
func TestCenter_DocumentRunIDOverridesDirections(t *testing.T) {
	c := newTestCenter(t, nil)
	d := doc("client", "server")
	d.Client.RunID = "stale-client"
	d.Server.RunID = "stale-server"
	assert.Error(t, c.Refresh(context.Background(), d)).Nil()
	assert.String(t, c.client.Config().RunID).Equal("drill-test")
	assert.String(t, c.server.Config().RunID).Equal("drill-test")
}

// TestCenter_DirectDocumentMayOmitRunID pins the direct path: a document built in
// code never went through the binder, may leave the document-level id empty, and
// then each direction keeps whatever code set — the data plane's "no run id"
// semantics.
func TestCenter_DirectDocumentMayOmitRunID(t *testing.T) {
	c := newTestCenter(t, nil)

	assert.Error(t, c.Refresh(context.Background(), kfault.Document{})).Nil()

	d := doc("client", "server")
	d.RunID = ""
	d.Client.RunID = "outbound-drill"
	d.Server.RunID = "inbound-drill"
	assert.Error(t, c.Refresh(context.Background(), d)).Nil()
	assert.String(t, c.client.Config().RunID).Equal("outbound-drill")
	assert.String(t, c.server.Config().RunID).Equal("inbound-drill")
}

// TestCenter_RefreshConcurrent exercises the per-root serialization: pushes and
// fires race under -race, and no push ever leaves a torn kernel.
func TestCenter_RefreshConcurrent(t *testing.T) {
	c := newTestCenter(t, nil)
	assert.Error(t, c.Refresh(context.Background(), doc("a", "as"))).Nil()
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = c.Refresh(context.Background(), doc("a", "as"))
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = c.client.Config()
			}
		}
	}()
	for range 5000 {
		_, _ = c.client.Fire(context.Background(), kfault.Target{Service: "x"})
	}
	close(stop)
	wg.Wait()
}

// documentYAML is one fully populated document, pinning which keys bind and how
// the kernel types (metadata match, action, durations) are spelled on the wire.
const documentYAML = `spring:
  fault:
    run-id: drill-yaml
    client:
      rules:
        - name: redis-errors
          service: redis:cache
          rate: 0.5
          error: generic
          latency: 10ms
          latency-jitter: 1ms
          max-duration: 1m
          max-affected: 100
          match:
            traffic: loadtest
          action:
            exact: insert
    server:
      rules:
        - name: inbound-slow
          service: gin
          rate: 1
          latency: 50ms
`

func TestParse_DocumentShape(t *testing.T) {
	doc, err := source.Parse[kfault.Document]("fault.yaml", []byte(documentYAML), "")
	assert.Error(t, err).Nil()
	assert.String(t, doc.RunID).Equal("drill-yaml")

	assert.Number(t, len(doc.Client.Rules)).Equal(1)
	c := doc.Client.Rules[0]
	assert.String(t, c.Name).Equal("redis-errors")
	assert.String(t, c.Service).Equal("redis:cache")
	assert.That(t, c.Rate).Equal(0.5)
	assert.String(t, c.ActionMatch.Exact).Equal("insert")
	assert.String(t, c.MetadataMatch["traffic"]).Equal("loadtest")
	assert.That(t, c.Latency).Equal(10 * time.Millisecond)
	assert.That(t, c.MaxDuration).Equal(time.Minute)
	assert.Number(t, int(c.MaxAffected)).Equal(100)

	assert.Number(t, len(doc.Server.Rules)).Equal(1)
	assert.String(t, doc.Server.Rules[0].Name).Equal("inbound-slow")
	assert.That(t, doc.Server.Rules[0].Latency).Equal(50 * time.Millisecond)
}

// TestParse_RejectsGovernanceOnlyDocument pins the root boundary: a governance
// document binds the ZERO fault document, and the required run-id then fails the
// binding — the guard is the field, not a transport-level prefix check.
func TestParse_RejectsGovernanceOnlyDocument(t *testing.T) {
	_, err := source.Parse[kfault.Document]("governance.properties",
		[]byte("client.resilience.timeout=1s\n"), "")
	assert.Error(t, err).NotNil()
}

// TestParse_EmptyDocumentSentinel pins the "off" spelling. The empty value for a
// list field is "", not []: flatten renders an empty slice as "[]" but the
// binder's single-string branch turns that into one zero element, whose missing
// name validation would reject. A disarm still names its run.
func TestParse_EmptyDocumentSentinel(t *testing.T) {
	const yaml = "spring:\n  fault:\n    run-id: drill-off\n    client:\n      rules: \"\"\n"
	doc, err := source.Parse[kfault.Document]("fault.yaml", []byte(yaml), "")
	assert.Error(t, err).Nil()
	assert.String(t, doc.RunID).Equal("drill-off")
	assert.Number(t, len(doc.Client.Rules)).Equal(0)

	// The same sentinel in properties spelling.
	doc, err = source.Parse[kfault.Document]("fault.properties",
		[]byte("spring.fault.run-id=drill-off\nspring.fault.client.rules=\n"), "")
	assert.Error(t, err).Nil()
	assert.Number(t, len(doc.Client.Rules)).Equal(0)
}

// TestParse_ValidatesAtBind pins binding-time validation: every constraint is a
// field-level expr tag judged by conf.Bind, so a document that cannot serve is
// rejected where it is READ, not only where it is pushed.
func TestParse_ValidatesAtBind(t *testing.T) {
	const yaml = "spring:\n  fault:\n    run-id: drill-unnamed\n    client:\n      rules:\n        - rate: 1\n"
	_, err := source.Parse[kfault.Document]("fault.yaml", []byte(yaml), "")
	assert.Error(t, err).NotNil() // rule has no name

	// A missing run id fails at the field's own required binding.
	_, err = source.Parse[kfault.Document]("fault.yaml", []byte("spring:\n  fault:\n    client:\n      rules: \"\"\n"), "")
	assert.Error(t, err).NotNil()
}
