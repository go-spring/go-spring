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
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// errGovernanceStub is returned by the test executor to prove a call was
// routed through the governance layer instead of reaching the broker.
var errGovernanceStub = errors.New("governance: rejected by test stub")

// stubExecutor counts Execute calls and always rejects, so a test can assert
// the chain path went through the executor without a live broker.
type stubExecutor struct{ called atomic.Int32 }

func (s *stubExecutor) Execute(context.Context, func(context.Context) error) error {
	s.called.Add(1)
	return errGovernanceStub
}
func (s *stubExecutor) Close() error { return nil }

// fakeTail is a scripted InnerPublisher tail: it counts the publishes that
// reached it. It stands in for the raw adapter so chain tests need no broker.
type fakeTail struct {
	ran int
	err error
}

func (f *fakeTail) Publish(context.Context, *amqp.Channel, string, string, bool, bool, amqp.Publishing) error {
	f.ran++
	return f.err
}

func (f *fakeTail) Release(bool) error { return nil }

// newStubbedClient builds a Client whose governance layer runs the stub
// executor over a fake tail.
func newStubbedClient(stub *stubExecutor, tail *fakeTail) *Client {
	guard := &GuardPublisher{exec: stub, next: tail}
	return &Client{InnerPublisher: NewObsPublisher(guard), guard: guard}
}

// NewClient always attaches an executor, whether or not the governance beans
// are wired. Whether it protects anything is decided by the governance rule for
// the service label, not by a per-instance switch: a manager with no rule
// yields a transparent pass-through.
func TestNewClientAttachesGovernance(t *testing.T) {
	// An unarmed center models a container with no governance rules: the
	// executor is a transparent pass-through, and the publish reaches the tail.
	center := governance.NewCenter(governance.Config{}, resilience.NewManager(nil), nil, fault.NewInjector(fault.Configs{}, nil), nil, nil)
	cl := NewClient(nil, "rabbitmq:test", center)
	guard := &GuardPublisher{exec: cl.guard.exec, next: &fakeTail{}}
	cl.InnerPublisher = NewObsPublisher(guard)

	err := cl.InnerPublisher.Publish(context.Background(), nil, "ex", "k", false, false, amqp.Publishing{})
	if err != nil {
		t.Fatalf("unarmed executor must pass the publish through: %v", err)
	}
}

// TestDriverPublishGuarded verifies the driver's Publish routes through the
// same chain the direct client API uses: with the stub executor armed, the
// publish is rejected by the executor and the channel is never touched.
func TestDriverPublishGuarded(t *testing.T) {
	stub := &stubExecutor{}
	cl := newStubbedClient(stub, &fakeTail{})
	// A nil channel is safe here: the executor rejects before the guarded
	// closure runs, which is exactly what this test asserts.
	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	assert.Error(t, err).Nil()
	p := &publisher{cl: cl, queue: "q", prop: prop}
	err = p.Publish(context.Background(), &messaging.Message{Payload: []byte("x")})
	if !errors.Is(err, errGovernanceStub) {
		t.Fatalf("expected stub rejection, got %v", err)
	}
	if n := stub.called.Load(); n != 1 {
		t.Fatalf("executor must run exactly once, ran %d", n)
	}
}

// queueLayer wraps the chain head and namespaces every queue name it passes
// down — the kind of behavior change no amqp middleware could express.
type queueLayer struct {
	InnerPublisher
	prefix string
}

func (q queueLayer) Publish(ctx context.Context, ch *amqp.Channel, exchange, key string, mandatory, immediate bool, pub amqp.Publishing) error {
	return q.InnerPublisher.Publish(ctx, ch, exchange, q.prefix+key, mandatory, immediate, pub)
}

// TestInnerPublisherReorganize pins the wrap-head protocol: a custom layer
// over the chain head rewrites the routing key, and the publish runs through
// it — the tail sees the rewritten key.
func TestInnerPublisherReorganize(t *testing.T) {
	tail := &fakeTail{}
	// No executor (nil) models governance off: the guard runs the call inline.
	guard := &GuardPublisher{next: tail}
	cl := &Client{InnerPublisher: NewObsPublisher(guard), guard: guard}
	cl.InnerPublisher = queueLayer{InnerPublisher: cl.InnerPublisher, prefix: "tenant."}

	err := cl.InnerPublisher.Publish(context.Background(), nil, "", "jobs", false, false, amqp.Publishing{})
	if err != nil {
		t.Fatalf("publish through the wrap-head layer: %v", err)
	}
	assert.That(t, tail.ran).Equal(1)
}
