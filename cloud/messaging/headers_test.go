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
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestNewMessageIDUniqueAndShaped(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id := NewMessageID()
		assert.String(t, id).Length(32)
		_, dup := seen[id]
		assert.That(t, dup).False()
		seen[id] = struct{}{}
	}
}

// fakePub records published messages and can fail on demand.
type fakePub struct {
	msgs []*Message
	fail int // 1-based index that fails
}

func (f *fakePub) Publish(_ context.Context, m *Message) error {
	f.msgs = append(f.msgs, m)
	if len(f.msgs) == f.fail {
		return errors.New("boom")
	}
	return nil
}

func (f *fakePub) Close() error { return nil }

// fakeBatchPub also implements BatchPublisher so PublishBatch takes the batch path.
type fakeBatchPub struct {
	fakePub
	batched [][]*Message
}

func (f *fakeBatchPub) PublishBatch(_ context.Context, msgs []*Message) error {
	f.batched = append(f.batched, msgs)
	f.msgs = append(f.msgs, msgs...)
	return nil
}

func TestPublishBatchFallsBackToLoop(t *testing.T) {
	p := &fakePub{}
	err := PublishBatch(context.Background(), p,
		&Message{Key: "a"}, &Message{Key: "b"}, &Message{Key: "c"})
	assert.Error(t, err).Nil()
	assert.Number(t, len(p.msgs)).Equal(3)
}

func TestPublishBatchStopsAtFirstError(t *testing.T) {
	p := &fakePub{fail: 2}
	err := PublishBatch(context.Background(), p,
		&Message{Key: "a"}, &Message{Key: "b"}, &Message{Key: "c"})
	assert.Error(t, err).NotNil()
	assert.Number(t, len(p.msgs)).Equal(2) // third never sent
}

func TestPublishBatchUsesBatchPath(t *testing.T) {
	p := &fakeBatchPub{}
	err := PublishBatch(context.Background(), p,
		&Message{Key: "a"}, &Message{Key: "b"})
	assert.Error(t, err).Nil()
	assert.Number(t, len(p.batched)).Equal(1)
	assert.Number(t, len(p.batched[0])).Equal(2)
}

func TestEnsureMessageID(t *testing.T) {
	m := &Message{}
	EnsureMessageID(m)
	id1 := m.Header(HeaderMessageID)
	assert.That(t, id1 != "").True()

	// An existing id is never overwritten.
	m.SetHeader(HeaderMessageID, "my-own-id")
	EnsureMessageID(m)
	assert.String(t, m.Header(HeaderMessageID)).Equal("my-own-id")
}
