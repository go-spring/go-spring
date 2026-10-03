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
// routed through the guard instead of reaching the broker.
var errGovernanceStub = errors.New("governance: rejected by test stub")

// stubExecutor counts Execute calls and always rejects, so a test can assert
// the driver path went through the executor without a live broker.
type stubExecutor struct{ called atomic.Int32 }

func (s *stubExecutor) Execute(context.Context, func(context.Context) error) error {
	s.called.Add(1)
	return errGovernanceStub
}
func (s *stubExecutor) Close() error { return nil }

// CreateClient attaches a guard while it builds the client — governance is
// applied in the constructor, not by a later starter step. Whether the executor
// protects anything is decided by the governance rule for the service label, not
// by a per-instance switch: with governance off the executor is a transparent
// pass-through, so attaching one costs a call frame and changes nothing else.
func TestCreateClientAttachesGuard(t *testing.T) {
	cl, err := DefaultDriver{}.CreateClient(context.Background(), Config{Brokers: "127.0.0.1:1"},
		cloud.ClientParams{Resilience: resilience.NewManager(nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	if _, ok := clientGuards.Load(cl); !ok {
		t.Fatal("CreateClient must attach an executor")
	}
	closeResilience(cl)
}

// AttachGovernance with the zero governance bundle still attaches an executor:
// the client degrades to the observe-only resilience.Unmanaged one rather than
// running bare, so a hand-built client is observed (with a one-time warning)
// exactly like a governed one until protection is armed.
func TestAttachGovernanceZeroBundleDegradesToUnmanaged(t *testing.T) {
	cl, err := kgo.NewClient(kgo.SeedBrokers("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	AttachGovernance(cl, "127.0.0.1:1", cloud.ClientParams{})
	if _, ok := clientGuards.Load(cl); !ok {
		t.Fatal("AttachGovernance must attach an executor even for the zero bundle")
	}
	closeResilience(cl)
}

// TestGuardedConsumeRoutesThroughGuard verifies an application owning its own
// poll loop can route a consumed record through the client's executor: with an
// executor attached, the record is rejected before the handler runs — the raw
// client's own PollFetches cannot be intercepted, so this is the one entry point
// that gives the raw consume path the guard, the messaging.* metrics and the
// access log.
func TestGuardedConsumeRoutesThroughGuard(t *testing.T) {
	cl, err := kgo.NewClient(kgo.SeedBrokers("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	stub := &stubExecutor{}
	clientGuards.Store(cl, &clientGuard{exec: stub, serviceLabel: "kafka:test"})
	defer clientGuards.Delete(cl)

	reached := false
	err = GuardedConsume(context.Background(), cl, &kgo.Record{Topic: "t"}, func(context.Context) error {
		reached = true
		return nil
	})
	if !errors.Is(err, errGovernanceStub) {
		t.Fatalf("expected stub rejection, got %v", err)
	}
	if reached {
		t.Fatal("handler must not run when the executor rejects the consume")
	}
	if n := stub.called.Load(); n != 1 {
		t.Fatalf("executor must run exactly once, ran %d", n)
	}
}

// TestDriverPublishGuarded verifies the driver's Publish routes through the
// resilience executor the direct client API uses: with an executor attached,
// the publish is rejected by the executor and never reaches the broker.
func TestDriverPublishGuarded(t *testing.T) {
	cl, err := kgo.NewClient(kgo.SeedBrokers("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	stub := &stubExecutor{}
	clientGuards.Store(cl, &clientGuard{exec: stub, serviceLabel: "kafka:test"})
	defer clientGuards.Delete(cl)

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
