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

package StarterNats

import (
	"context"
	"time"

	"errors"
	"github.com/nats-io/nats.go"
	"go-spring.org/cloud/chain"
	"testing"

	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// fakeTail is a scripted InnerConn tail: it counts the calls that reached it
// and answers from a settable error. It stands in for the raw adapter so chain
// tests need no live nats server.
type fakeTail struct {
	ran int
	err error
}

func (f *fakeTail) PublishMsg(msg *nats.Msg) error {
	f.ran++
	return f.err
}

func (f *fakeTail) PublishMsgContext(ctx context.Context, msg *nats.Msg) error {
	f.ran++
	return f.err
}

func (f *fakeTail) RequestGuarded(context.Context, string, []byte, time.Duration) (*nats.Msg, error) {
	f.ran++
	return nil, f.err
}

func (f *fakeTail) Release(bool) error { return nil }

// newGuardedConn builds a Conn whose chain is identity over governance over a
// fake tail, with the governance layer's executor from the default resilience
// driver.
func newGuardedConn(t *testing.T, p resilience.ClientPolicy) (*Conn, *fakeTail) {
	d := resilience.NewDefaultDriver(nil)
	exec, err := d.NewClientExecutor("svc", p)
	assert.Error(t, err).Nil()
	tail := &fakeTail{}
	guard := &GuardConn{exec: exec, next: tail}
	return &Conn{InnerConn: NewObsConn(guard), guard: guard}, tail
}

// TestChainPassThrough proves the degraded stance: a chain whose governance
// layer has no executor runs the call inline and returns its result unchanged.
func TestChainPassThrough(t *testing.T) {
	tail := &fakeTail{}
	guard := &GuardConn{next: tail}
	c := &Conn{InnerConn: NewObsConn(guard), guard: guard}

	boom := errors.New("boom")
	tail.err = boom
	assert.Error(t, c.PublishMsgContext(context.Background(), &nats.Msg{Subject: "t.s"})).Is(boom)
	assert.That(t, tail.ran).Equal(1)
}

// TestChainRateLimit confirms the flow-control path: once the burst is spent,
// further publishes are rejected without reaching the tail.
func TestChainRateLimit(t *testing.T) {
	c, tail := newGuardedConn(t, resilience.ClientPolicy{RateLimit: 1, Burst: 2})
	pub := func() error {
		return c.PublishMsgContext(context.Background(), &nats.Msg{Subject: "t.s"})
	}

	assert.Error(t, pub()).Nil()
	assert.Error(t, pub()).Nil()
	assert.Error(t, pub()).Is(chain.ErrRateLimited)
	assert.That(t, tail.ran).Equal(2) // the rejected call never reached the tail
}

// TestChainCircuitOpen confirms genuine failures still open the circuit and the
// rejection short-circuits the next call before the tail runs.
func TestChainCircuitOpen(t *testing.T) {
	c, tail := newGuardedConn(t, resilience.ClientPolicy{ErrorThreshold: 2})
	tail.err = errors.New("connection reset")
	pub := func() error {
		return c.PublishMsgContext(context.Background(), &nats.Msg{Subject: "t.s"})
	}

	assert.Error(t, pub()).Is(tail.err)
	assert.Error(t, pub()).Is(tail.err)

	// Breaker is now open: a call that would succeed is rejected upfront.
	tail.err = nil
	err := pub()
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
	assert.That(t, tail.ran).Equal(2)
}
