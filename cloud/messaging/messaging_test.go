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

package messaging

import (
	"context"
	"sync"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestMessage_Headers(t *testing.T) {
	var m Message
	assert.String(t, m.Header("k")).Equal("") // nil-safe
	m.SetHeader("traceparent", "00-abc-def-01")
	assert.String(t, m.Header("traceparent")).Equal("00-abc-def-01")

	var nilMsg *Message
	assert.String(t, nilMsg.Header("k")).Equal("")
}

// memDriver is an in-process fake driver used to exercise the abstraction and
// prove a round-trip without any real broker.
type memDriver struct {
	mu   sync.Mutex
	subs map[string][]Handler
}

func newMemDriver() *memDriver { return &memDriver{subs: map[string][]Handler{}} }

func (b *memDriver) NewPublisher(_ context.Context, dest string) (Publisher, error) {
	return &memPublisher{b: b, dest: dest}, nil
}

func (b *memDriver) NewSubscriber(_ context.Context, source, _ string) (Subscriber, error) {
	return &memSubscriber{b: b, source: source}, nil
}

func (b *memDriver) deliver(ctx context.Context, dest string, msg *Message) error {
	b.mu.Lock()
	handlers := append([]Handler(nil), b.subs[dest]...)
	b.mu.Unlock()
	for _, h := range handlers {
		if err := h(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

type memPublisher struct {
	b    *memDriver
	dest string
}

func (p *memPublisher) Publish(ctx context.Context, msg *Message) error {
	return p.b.deliver(ctx, p.dest, msg)
}
func (p *memPublisher) Close() error { return nil }

type memSubscriber struct {
	b      *memDriver
	source string
}

func (s *memSubscriber) Subscribe(_ context.Context, handler Handler) error {
	s.b.mu.Lock()
	s.b.subs[s.source] = append(s.b.subs[s.source], handler)
	s.b.mu.Unlock()
	return nil
}
func (s *memSubscriber) Close() error { return nil }

func TestDriver_RoundTrip(t *testing.T) {
	b := newMemDriver()
	ctx := context.Background()

	sub, err := b.NewSubscriber(ctx, "orders", "workers")
	assert.Error(t, err).Nil()
	defer sub.Close()

	var got *Message
	err = sub.Subscribe(ctx, func(_ context.Context, msg *Message) error {
		got = msg
		return nil
	})
	assert.Error(t, err).Nil()

	pub, err := b.NewPublisher(ctx, "orders")
	assert.Error(t, err).Nil()
	defer pub.Close()

	sent := &Message{Key: "o-1", Payload: []byte("hello")}
	sent.SetHeader("traceparent", "00-abc-def-01")
	assert.Error(t, pub.Publish(ctx, sent)).Nil()

	assert.That(t, got).NotNil()
	assert.String(t, got.Key).Equal("o-1")
	assert.String(t, string(got.Payload)).Equal("hello")
	assert.String(t, got.Header("traceparent")).Equal("00-abc-def-01")
}

func TestRegistry(t *testing.T) {
	RegisterDriver("mem", newMemDriver())

	b, err := GetDriver("mem")
	assert.Error(t, err).Nil()
	assert.That(t, b).NotNil()

	_, err = GetDriver("missing")
	assert.Error(t, err).Matches("no driver registered as \"missing\"")

	assert.Panic(t, func() { RegisterDriver("mem", newMemDriver()) }, "already registered")
	assert.Panic(t, func() { RegisterDriver("", newMemDriver()) }, "empty name")
	assert.Panic(t, func() { RegisterDriver("nil-b", nil) }, "nil driver")
}
