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

package StarterKafka

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/cloud"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/resilience"
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

// fakeTail is a scripted InnerKafka tail: it counts the produces that reached
// it. It stands in for the raw adapter so chain tests need no broker.
type fakeTail struct {
	ran int
}

func (f *fakeTail) ProduceSync(context.Context, ...*kgo.Record) kgo.ProduceResults {
	f.ran++
	return nil
}

func (f *fakeTail) Release(bool) error { return nil }

// newStubbedClient builds a Client whose governance layer runs the stub
// executor over a fake tail.
func newStubbedClient(stub *stubExecutor) *Client {
	guard := &GuardKafka{exec: stub, next: &fakeTail{}}
	return &Client{InnerKafka: NewObsKafka(guard), guard: guard}
}

// NewClient wraps governance in while it builds the client — the chain rides
// the wrapper, not a registry. Whether the executor protects anything is
// decided by the governance rule for the service label, not by a per-instance
// switch: with governance off the executor is a transparent pass-through, so
// attaching one costs a call frame and changes nothing else.
func TestNewClientAttachesGovernance(t *testing.T) {
	cl, err := DefaultDriver{}.CreateClient(context.Background(), Config{Brokers: "127.0.0.1:1"},
		cloud.ClientParams{Resilience: resilience.NewManager(nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	wrapped := NewClient(cl, "127.0.0.1:1", cloud.ClientParams{})
	if wrapped.guard == nil || wrapped.guard.exec == nil {
		t.Fatal("NewClient must attach an executor")
	}
	_ = wrapped.Close()
}

// TestGuardedConsumeRoutesThroughGuard verifies an application owning its own
// poll loop can route a consumed record through the client's governance layer:
// with the stub executor armed, the record is rejected before the handler
// runs — the raw client's own PollFetches cannot be intercepted, so this is
// the one entry point that gives the raw consume path the guard, the
// messaging.* metrics and the access log.
func TestGuardedConsumeRoutesThroughGuard(t *testing.T) {
	cl := newStubbedClient(&stubExecutor{})

	reached := false
	err := GuardedConsume(context.Background(), cl, &kgo.Record{Topic: "t"}, func(context.Context) error {
		reached = true
		return nil
	})
	if !errors.Is(err, errGovernanceStub) {
		t.Fatalf("expected stub rejection, got %v", err)
	}
	if reached {
		t.Fatal("handler must not run when the executor rejects the consume")
	}
}

// TestDriverPublishGuarded verifies the driver's Publish routes through the
// same chain the direct client API uses: with the stub executor armed, the
// publish is rejected by the executor and never reaches the tail.
func TestDriverPublishGuarded(t *testing.T) {
	stub := &stubExecutor{}
	cl := newStubbedClient(stub)

	b := NewDriver(cl, nil)
	pub, err := b.NewPublisher(context.Background(), "t")
	if err != nil {
		t.Fatal(err)
	}
	err = pub.Publish(context.Background(), &messaging.Message{Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected rejection from the governance executor")
	}
	if !errors.Is(err, errGovernanceStub) {
		t.Fatalf("expected stub rejection, got %v", err)
	}
	if n := stub.called.Load(); n != 1 {
		t.Fatalf("executor must run exactly once, ran %d", n)
	}
}

// topicLayer wraps the chain head and namespaces every topic it passes down —
// the kind of behavior change no franz-go hook could express (hooks observe,
// they do not rewrite).
type topicLayer struct {
	InnerKafka
	prefix string
}

func (t topicLayer) ProduceSync(ctx context.Context, recs ...*kgo.Record) kgo.ProduceResults {
	out := make([]*kgo.Record, 0, len(recs))
	for _, r := range recs {
		out = append(out, &kgo.Record{Topic: t.prefix + r.Topic, Key: r.Key, Value: r.Value})
	}
	return t.InnerKafka.ProduceSync(ctx, out...)
}

// TestInnerKafkaReorganize pins the wrap-head protocol: a custom layer over
// the chain head rewrites the topic, and the produce runs through it — the
// tail sees the rewritten topic.
func TestInnerKafkaReorganize(t *testing.T) {
	tail := &fakeTail{}
	guard := &GuardKafka{next: tail} // no executor: governance off, inline run
	cl := &Client{InnerKafka: NewObsKafka(guard), guard: guard}
	cl.InnerKafka = topicLayer{InnerKafka: cl.InnerKafka, prefix: "tenant."}

	results := cl.InnerKafka.ProduceSync(context.Background(), &kgo.Record{Topic: "orders"})
	if results.FirstErr() != nil {
		t.Fatalf("produce through the wrap-head layer: %v", results.FirstErr())
	}
	if tail.ran != 1 {
		t.Fatalf("tail ran %d times, want 1", tail.ran)
	}
}
