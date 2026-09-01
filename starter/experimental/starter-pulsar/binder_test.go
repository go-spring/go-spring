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
	"sync"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/cloud/messaging"
	"go-spring.org/stdlib/testing/assert"
)

// fakeMessage is a pulsar.Message that only implements what the binder's
// consume loop touches (topic, properties, payload); everything else panics
// via the nil embedded interface if reached unexpectedly.
type fakeMessage struct {
	pulsar.Message
}

func (fakeMessage) Topic() string                 { return "t-test" }
func (fakeMessage) Properties() map[string]string { return nil }
func (fakeMessage) Key() string                   { return "k-1" }
func (fakeMessage) Payload() []byte               { return []byte("hello") }
func (fakeMessage) PublishTime() time.Time        { return time.Unix(0, 0) }

// fakeConsumer delivers exactly one message, then mirrors context
// cancellation the way a real consumer does when Close cancels the loop.
type fakeConsumer struct {
	pulsar.Consumer

	mu        sync.Mutex
	delivered bool
	ackCalls  int
	ackErr    error // returned by Ack; exercises the warn path when non-nil
}

func (f *fakeConsumer) Receive(ctx context.Context) (pulsar.Message, error) {
	f.mu.Lock()
	first := !f.delivered
	f.delivered = true
	f.mu.Unlock()
	if first {
		return fakeMessage{}, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakeConsumer) Ack(pulsar.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ackCalls++
	return f.ackErr
}

func (f *fakeConsumer) Nack(pulsar.Message) {}
func (f *fakeConsumer) Close()              {}

// TestSubscriberAckErrorCoversWarnPath drives the consume loop end to end
// against a fake consumer whose Ack fails, proving the loop still calls Ack,
// logs the failure instead of panicking, and shuts down cleanly.
func TestSubscriberAckErrorCoversWarnPath(t *testing.T) {
	fc := &fakeConsumer{ackErr: errors.New("ack refused")}
	s := &subscriber{c: fc}

	done := make(chan struct{})
	err := s.Subscribe(context.Background(), func(_ context.Context, m *messaging.Message) error {
		defer close(done)
		assert.That(t, len(m.Payload) > 0).True()
		return nil
	})
	assert.Error(t, err).Nil()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler was never invoked")
	}
	assert.Error(t, s.Close()).Nil()

	fc.mu.Lock()
	defer fc.mu.Unlock()
	assert.That(t, fc.ackCalls == 1).True()
}
