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
	"time"

	"go-spring.org/stdlib/testing/assert"
)

// countingHandler fails the first failN calls then succeeds, recording the
// number of attempts it saw.
type countingHandler struct {
	failN  int
	calls  int
	err    error
}

func (h *countingHandler) handle(_ context.Context, _ *Message) error {
	h.calls++
	if h.calls <= h.failN {
		return h.err
	}
	return nil
}

func TestRetrySucceedsAfterTransientFailures(t *testing.T) {
	h := &countingHandler{failN: 2, err: errors.New("transient")}
	// MaxRetries 2 = three attempts total; the handler recovers on the third.
	err := Retry(h.handle, RetryPolicy{MaxRetries: 2})(context.Background(), &Message{})
	assert.That(t, err).Nil()
	assert.That(t, h.calls).Equal(3)
}

func TestRetryExhaustsAndReturnsLastError(t *testing.T) {
	boom := errors.New("boom")
	h := &countingHandler{failN: 100, err: boom}
	err := Retry(h.handle, RetryPolicy{MaxRetries: 2})(context.Background(), &Message{})
	assert.That(t, err).NotNil()
	// The wrapped error both reports the attempt count and unwraps to the
	// handler's own error, so a DLQ header keeps the root cause readable.
	assert.That(t, errors.Is(err, boom)).True()
	assert.That(t, h.calls).Equal(3)
}

func TestRetryZeroPolicyIsSingleAttempt(t *testing.T) {
	// The zero RetryPolicy retries nothing: one call, then fail.
	h := &countingHandler{failN: 1, err: errors.New("x")}
	err := Retry(h.handle, RetryPolicy{})(context.Background(), &Message{})
	assert.That(t, err).NotNil()
	assert.That(t, h.calls).Equal(1)
}

func TestRetryHonoursContextCancellation(t *testing.T) {
	h := &countingHandler{failN: 100, err: errors.New("x")}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := Retry(h.handle, RetryPolicy{MaxRetries: 100, InitialInterval: 50 * time.Millisecond})(ctx, &Message{})
	assert.That(t, err).NotNil()
	// Cancelled mid-backoff, not after 100 retries x 50ms.
	assert.That(t, time.Since(start) < time.Second).True()
}

func TestBackoffGrowthAndCap(t *testing.T) {
	p := RetryPolicy{InitialInterval: 10 * time.Millisecond, Multiplier: 2, MaxInterval: 40 * time.Millisecond}
	// 10ms -> 20ms -> 40ms -> capped at 40ms.
	assert.That(t, p.backoff(0)).Equal(10 * time.Millisecond)
	assert.That(t, p.backoff(1)).Equal(20 * time.Millisecond)
	assert.That(t, p.backoff(2)).Equal(40 * time.Millisecond)
	assert.That(t, p.backoff(3)).Equal(40 * time.Millisecond)
	// Multiplier below 1 is treated as 1 (constant backoff); no interval means
	// immediate retries.
	noGrow := RetryPolicy{InitialInterval: 10 * time.Millisecond, Multiplier: 0.5}
	assert.That(t, noGrow.backoff(0)).Equal(10 * time.Millisecond)
	assert.That(t, noGrow.backoff(5)).Equal(10 * time.Millisecond)
	assert.That(t, RetryPolicy{}.backoff(0)).Equal(time.Duration(0))
}

// fakePublisher records everything published to it.
type fakePublisher struct {
	msgs []*Message
	err  error
}

func (p *fakePublisher) Publish(_ context.Context, msg *Message) error {
	if p.err != nil {
		return p.err
	}
	m := *msg
	p.msgs = append(p.msgs, &m)
	return nil
}

func (p *fakePublisher) Close() error { return nil }

func TestDeadLetterRoutesExhaustedMessage(t *testing.T) {
	boom := errors.New("boom")
	h := &countingHandler{failN: 100, err: boom}
	dlq := &fakePublisher{}

	err := DeadLetter(h.handle, dlq, RetryPolicy{MaxRetries: 1})(context.Background(), &Message{
		Key:     "order-1",
		Payload: []byte("payload"),
		Headers: map[string]string{"traceparent": "00-..."},
	})
	// The original delivery is acked (nil): the copy went to the DLQ.
	assert.That(t, err).Nil()
	assert.That(t, len(dlq.msgs)).Equal(1)

	m := dlq.msgs[0]
	assert.That(t, string(m.Payload)).Equal("payload")
	// Original headers ride along plus the DLQ metadata triple.
	assert.That(t, m.Header("traceparent")).Equal("00-...")
	assert.That(t, m.Header(HeaderDLQRetries)).Equal("2")
	assert.That(t, m.Header(HeaderDLQKey)).Equal("order-1")
	assert.That(t, m.Header(HeaderDLQError) != "").True()
}

func TestDeadLetterAcksWhenHandlerSucceeds(t *testing.T) {
	h := &countingHandler{failN: 1, err: errors.New("transient")}
	dlq := &fakePublisher{}
	// Recovers on the retry: nothing dead-letters.
	err := DeadLetter(h.handle, dlq, RetryPolicy{MaxRetries: 1})(context.Background(), &Message{})
	assert.That(t, err).Nil()
	assert.That(t, len(dlq.msgs)).Equal(0)
}

func TestDeadLetterPublishFailureNacksOriginal(t *testing.T) {
	h := &countingHandler{failN: 100, err: errors.New("boom")}
	dlqErr := errors.New("dlq down")
	dlq := &fakePublisher{err: dlqErr}
	// Losing a dead letter is worse than redelivering: the original error is
	// returned so the broker nack path takes over.
	err := DeadLetter(h.handle, dlq, RetryPolicy{MaxRetries: 0})(context.Background(), &Message{})
	assert.That(t, err).NotNil()
	assert.That(t, errors.Is(err, dlqErr)).True()
}
