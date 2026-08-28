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

package StarterMQTT

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/cloud/experimental/messaging"
	"go-spring.org/cloud/governance/resilience"
)

// errGovernanceStub is returned by the test executor to prove a call was
// routed through the guard instead of reaching the broker.
var errGovernanceStub = errors.New("governance: rejected by test stub")

// stubExecutor counts Execute calls and always rejects, so a test can assert
// the binder path went through the executor without a live broker.
type stubExecutor struct{ called atomic.Int32 }

func (s *stubExecutor) Execute(context.Context, string, func(context.Context) error) error {
	s.called.Add(1)
	return errGovernanceStub
}
func (s *stubExecutor) Close() error                    { return nil }
func (s *stubExecutor) Refresh(resilience.Policy) error { return nil }

// fakeToken is a completed paho token.
type fakeToken struct{ err error }

func (t *fakeToken) Wait() bool                     { return true }
func (t *fakeToken) WaitTimeout(time.Duration) bool { return true }
func (t *fakeToken) Error() error                   { return t.err }
func (t *fakeToken) Done() <-chan struct{}          { ch := make(chan struct{}); close(ch); return ch }

// fakeMQTTClient records whether the underlying Publish ran.
type fakeMQTTClient struct {
	mqtt.Client
	published atomic.Int32
}

func (f *fakeMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	f.published.Add(1)
	return &fakeToken{}
}

// TestApplyResilienceToggle verifies the per-instance governance opt-out: with
// Governance=false no executor is attached (all call paths degrade to bare
// calls); with the default true an executor is registered.
func TestApplyResilienceToggle(t *testing.T) {
	cl := &fakeMQTTClient{}

	if err := applyResilience(Config{Governance: false}, cl, "mqtt:test"); err != nil {
		t.Fatal(err)
	}
	if _, ok := resilienceExecs.Load(cl); ok {
		t.Fatal("governance=false must not attach an executor")
	}

	if err := applyResilience(Config{Governance: true}, cl, "mqtt:test"); err != nil {
		t.Fatal(err)
	}
	if _, ok := resilienceExecs.Load(cl); !ok {
		t.Fatal("governance=true (default) must attach an executor")
	}
	closeResilience(cl)
}

// TestBinderPublishGuarded verifies the binder's Publish routes through the
// resilience executor the direct client API uses: with an executor attached,
// the publish is rejected and paho's Publish never runs; with no executor
// (governance off) the underlying client is published to directly.
func TestBinderPublishGuarded(t *testing.T) {
	cl := &fakeMQTTClient{}
	stub := &stubExecutor{}
	resilienceExecs.Store(cl, stub)
	resilienceResources.Store(cl, "mqtt:test")

	b := NewBinder(cl)
	pub, err := b.NewPublisher(context.Background(), "t")
	if err != nil {
		t.Fatal(err)
	}
	if err = pub.Publish(context.Background(), &messaging.Message{Payload: []byte("x")}); !errors.Is(err, errGovernanceStub) {
		t.Fatalf("expected stub rejection, got %v", err)
	}
	if n := stub.called.Load(); n != 1 {
		t.Fatalf("executor must run exactly once, ran %d", n)
	}
	if n := cl.published.Load(); n != 0 {
		t.Fatalf("guarded publish must not reach paho, reached %d times", n)
	}

	// Governance off (no executor attached): the publish goes straight through.
	resilienceExecs.Delete(cl)
	resilienceResources.Delete(cl)
	if err = pub.Publish(context.Background(), &messaging.Message{Payload: []byte("x")}); err != nil {
		t.Fatalf("bare publish must succeed on the fake client, got %v", err)
	}
	if n := cl.published.Load(); n != 1 {
		t.Fatalf("bare publish must reach paho exactly once, reached %d times", n)
	}
}
