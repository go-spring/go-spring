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
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// errGovernanceStub is returned by the test executor to prove a call was
// routed through the guard instead of reaching the broker.
var errGovernanceStub = errors.New("governance: rejected by test stub")

// stubExecutor counts Execute calls and always rejects, so a test can assert
// the driver path went through the executor without a live broker.
type stubExecutor struct{ called atomic.Int32 }

func (s *stubExecutor) Execute(context.Context, func(context.Context) error) error {
	s.called.Add(1)
	return errGovernanceStub
}
func (s *stubExecutor) Close() error                          { return nil }
func (s *stubExecutor) Refresh(resilience.ClientPolicy) error { return nil }

// applyResilience always attaches an executor, whether or not the governance
// beans are wired. Whether it protects anything is decided by the governance
// rule for the service label, not by a per-instance switch: a manager with no
// rule yields a transparent pass-through, so attaching one costs a call frame
// and changes nothing else.
func TestApplyResilienceAttachesGuard(t *testing.T) {
	conn := &amqp.Connection{}

	// A nil manager models the standalone caller with no container.
	if err := applyResilience(conn, "rabbitmq:test", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := clientGuards.Load(conn); !ok {
		t.Fatal("applyResilience must attach an executor")
	}
	closeResilience(conn)
}

// TestDriverPublishGuarded verifies the driver's Publish routes through the
// resilience executor the direct client API uses: with an executor attached,
// the publish is rejected by the executor and the channel is never touched.
func TestDriverPublishGuarded(t *testing.T) {
	conn := &amqp.Connection{}
	stub := &stubExecutor{}
	clientGuards.Store(conn, &clientGuard{exec: stub, serviceLabel: "rabbitmq:test"})
	defer func() {
		clientGuards.Delete(conn)
	}()
	// A nil channel is safe here: the executor rejects before the guarded
	// closure runs, which is exactly what this test asserts.
	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	assert.Error(t, err).Nil()
	p := &publisher{conn: conn, queue: "q", prop: prop}
	err = p.Publish(context.Background(), &messaging.Message{Payload: []byte("x")})
	if !errors.Is(err, errGovernanceStub) {
		t.Fatalf("expected stub rejection, got %v", err)
	}
	if n := stub.called.Load(); n != 1 {
		t.Fatalf("executor must run exactly once, ran %d", n)
	}
}
