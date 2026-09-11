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

package StarterPulsar

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/messaging"
)

// errGovernanceStub is returned by the test executor to prove a call was
// routed through the guard instead of reaching the broker.
var errGovernanceStub = errors.New("governance: rejected by test stub")

// stubExecutor counts Execute calls and always rejects, so a test can assert
// the driver path went through the executor without a live broker.
type stubExecutor struct{ called atomic.Int32 }

func (s *stubExecutor) Execute(context.Context, string, func(context.Context) error) error {
	s.called.Add(1)
	return errGovernanceStub
}
func (s *stubExecutor) Close() error                    { return nil }
func (s *stubExecutor) Refresh(resilience.Policy) error { return nil }

// fakePulsarClient is a map-key-only stand-in for the client bean; no method
// is ever invoked because the executor rejects first.
type fakePulsarClient struct{ pulsar.Client }

// fakePulsarProducer satisfies the producer surface the driver touches before
// the guard (Topic) and would touch inside it (never, in these tests).
type fakePulsarProducer struct {
	pulsar.Producer
	topic string
}

func (f *fakePulsarProducer) Topic() string { return f.topic }

// TestApplyResilienceToggle verifies the per-instance governance opt-out: with
// Governance=false no executor is attached (all call paths degrade to bare
// calls); with the default true an executor is registered.
func TestApplyResilienceToggle(t *testing.T) {
	cl := &fakePulsarClient{}

	if err := applyResilience(Config{Governance: false}, cl, "pulsar:test"); err != nil {
		t.Fatal(err)
	}
	if _, ok := clientGuards.Load(cl); ok {
		t.Fatal("governance=false must not attach an executor")
	}

	if err := applyResilience(Config{Governance: true}, cl, "pulsar:test"); err != nil {
		t.Fatal(err)
	}
	if _, ok := clientGuards.Load(cl); !ok {
		t.Fatal("governance=true (default) must attach an executor")
	}
	closeResilience(cl)
}

// TestDriverPublishGuarded verifies the driver's Publish routes through the
// resilience executor the direct client API uses: with an executor attached,
// the send is rejected by the executor and the producer's Send never runs.
func TestDriverPublishGuarded(t *testing.T) {
	cl := &fakePulsarClient{}
	stub := &stubExecutor{}
	clientGuards.Store(cl, &clientGuard{exec: stub, resource: "pulsar:test"})
	defer func() {
		clientGuards.Delete(cl)
	}()
	p := &publisher{cl: cl, p: &fakePulsarProducer{topic: "t"}}
	err := p.Publish(context.Background(), &messaging.Message{Payload: []byte("x")})
	if !errors.Is(err, errGovernanceStub) {
		t.Fatalf("expected stub rejection, got %v", err)
	}
	if n := stub.called.Load(); n != 1 {
		t.Fatalf("executor must run exactly once, ran %d", n)
	}
}
