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
	"fmt"
	"math"
	"time"
)

// Header names stamped by [DeadLetter] on the copy routed to the dead-letter
// destination, so a consumer of the DLQ can tell why the message landed there.
const (
	// HeaderDLQError carries the final handler error that exhausted delivery.
	HeaderDLQError = "x-dlq-error"

	// HeaderDLQRetries carries how many attempts were made before giving up
	// ("3" means one initial try plus two retries).
	HeaderDLQRetries = "x-dlq-retries"

	// HeaderDLQKey carries the original ordering key of the dead-lettered
	// message, since the DLQ copy's own Key field carries the destination
	// routing key (usually the same source name).
	HeaderDLQKey = "x-dlq-key"
)

// RetryPolicy bounds in-process redelivery before a message is nacked (or
// dead-lettered, when composed under [DeadLetter]). It is deliberately a small
// value struct independent of the governance resilience Policy: a consumer's
// retry loop is a delivery concern, not a call-path circuit — retries here
// re-run the handler on the SAME delivery, so breaker/rate-limit semantics do
// not apply, only bounded attempts with backoff.
//
// The zero value retries nothing: MaxRetries 0 means one attempt, then fail.
type RetryPolicy struct {
	// MaxRetries is the number of retries AFTER the initial attempt. Negative
	// is treated as zero.
	MaxRetries int

	// InitialInterval is the backoff before the first retry; each subsequent
	// retry multiplies it by Multiplier up to MaxInterval. Zero or negative
	// disables waiting (immediate retries).
	InitialInterval time.Duration

	// Multiplier scales the backoff per retry (1.5 is a common choice). Values
	// below 1 are treated as 1.
	Multiplier float64

	// MaxInterval caps a single backoff. Zero means no cap.
	MaxInterval time.Duration
}

// backoff returns the wait before attempt n (0-based retry index).
func (p RetryPolicy) backoff(n int) time.Duration {
	if p.InitialInterval <= 0 {
		return 0
	}
	m := p.Multiplier
	if m < 1 {
		m = 1
	}
	d := time.Duration(float64(p.InitialInterval) * math.Pow(m, float64(n)))
	if p.MaxInterval > 0 && d > p.MaxInterval {
		d = p.MaxInterval
	}
	return d
}

// Retry returns a Handler that retries h up to p.MaxRetries times with
// exponential backoff when it returns an error. A success on any attempt acks
// (returns nil); once retries are exhausted the last error is returned, so the
// binder's own failure path (nack / broker redelivery) takes over.
//
// It composes with [SafeHandler]: wrap Retry OUTSIDE SafeHandler
// (Retry(SafeHandler(h), p)) so a panic is converted to an error once and then
// retried like any other failure; the reverse order would re-panic per attempt.
func Retry(h Handler, p RetryPolicy) Handler {
	return func(ctx context.Context, msg *Message) error {
		var err error
		for attempt := 0; ; attempt++ {
			if err = h(ctx, msg); err == nil {
				return nil
			}
			if attempt >= p.MaxRetries {
				return fmt.Errorf("messaging: delivery failed after %d attempt(s): %w", attempt+1, err)
			}
			if d := p.backoff(attempt); d > 0 {
				select {
				case <-time.After(d):
				case <-ctx.Done():
					return fmt.Errorf("messaging: retry cancelled: %w", err)
				}
			}
		}
	}
}

// DeadLetter returns a Handler that gives h up to p.MaxRetries retries and,
// when delivery still fails, publishes the message to dlq instead of returning
// the error: the original delivery is acked (nil returned) so the broker stops
// redelivering it, and a copy carrying the failure reason (headers
// [HeaderDLQError] / [HeaderDLQRetries] / [HeaderDLQKey]) goes to the
// dead-letter destination for inspection or replay.
//
// If the DLQ publish itself fails the original error is returned (nack /
// broker redelivery) — losing a dead letter is worse than redelivering.
//
// Typical wiring, with dlq a Publisher bound to "<queue>.dlq":
//
//	sub.Subscribe(ctx, messaging.DeadLetter(messaging.SafeHandler(h), dlq, messaging.RetryPolicy{MaxRetries: 2, InitialInterval: 100 * time.Millisecond}))
//
// This is the binder-neutral DLQ contract: brokers with native dead-lettering
// (RabbitMQ DLX, RocketMQ DLQ topics) can be configured instead, at which point
// DeadLetter is unnecessary — p exhausted means the handler returns its error
// and the broker's own DLX routing takes over. Both routes carry the original
// payload; only the failure metadata differs (broker headers vs these).
func DeadLetter(h Handler, dlq Publisher, p RetryPolicy) Handler {
	return func(ctx context.Context, msg *Message) error {
		err := Retry(h, p)(ctx, msg)
		if err == nil {
			return nil
		}
		dlqMsg := &Message{
			Payload:   msg.Payload,
			Headers:   make(map[string]string, len(msg.Headers)+3),
			Timestamp: time.Now(),
		}
		for k, v := range msg.Headers {
			dlqMsg.Headers[k] = v
		}
		dlqMsg.Headers[HeaderDLQError] = err.Error()
		dlqMsg.Headers[HeaderDLQRetries] = fmt.Sprintf("%d", p.MaxRetries+1)
		dlqMsg.Headers[HeaderDLQKey] = msg.Key
		if perr := dlq.Publish(ctx, dlqMsg); perr != nil {
			return fmt.Errorf("messaging: dead-letter publish failed (original error: %v): %w", err, perr)
		}
		return nil
	}
}
