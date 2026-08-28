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

package StarterRabbitMQ

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
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

// TestApplyResilienceToggle verifies the per-instance governance opt-out: with
// Governance=false no executor is attached (all call paths degrade to bare
// calls); with the default true an executor is registered.
func TestApplyResilienceToggle(t *testing.T) {
	conn := &amqp.Connection{}

	if err := applyResilience(Config{Governance: false}, conn, "rabbitmq:test"); err != nil {
		t.Fatal(err)
	}
	if _, ok := resilienceExecs.Load(conn); ok {
		t.Fatal("governance=false must not attach an executor")
	}

	if err := applyResilience(Config{Governance: true}, conn, "rabbitmq:test"); err != nil {
		t.Fatal(err)
	}
	if _, ok := resilienceExecs.Load(conn); !ok {
		t.Fatal("governance=true (default) must attach an executor")
	}
	closeResilience(conn)
}

// TestBinderPublishGuarded verifies the binder's Publish routes through the
// resilience executor the direct client API uses: with an executor attached,
// the publish is rejected by the executor and the channel is never touched.
func TestBinderPublishGuarded(t *testing.T) {
	conn := &amqp.Connection{}
	stub := &stubExecutor{}
	resilienceExecs.Store(conn, stub)
	resilienceResources.Store(conn, "rabbitmq:test")
	defer func() {
		resilienceExecs.Delete(conn)
		resilienceResources.Delete(conn)
	}()

	// A nil channel is safe here: the executor rejects before the guarded
	// closure runs, which is exactly what this test asserts.
	p := &publisher{conn: conn, queue: "q"}
	err := p.Publish(context.Background(), &messaging.Message{Payload: []byte("x")})
	if !errors.Is(err, errGovernanceStub) {
		t.Fatalf("expected stub rejection, got %v", err)
	}
	if n := stub.called.Load(); n != 1 {
		t.Fatalf("executor must run exactly once, ran %d", n)
	}
}
