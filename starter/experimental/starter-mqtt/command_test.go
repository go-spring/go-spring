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

package StarterMQTT

import (
	"context"
	"testing"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
)

// The guarded entries no longer emit: they DECLARE the operation on the ctx and
// route the call through the resilience executor, which is where the span,
// metrics and access log are produced. These tests drive that seam without a
// live broker — the fake client's Publish is a stub — and with a real resilience
// wrapper over a capturing inner, so the declaration is observable exactly where
// the emitter would read it.

// captureExecutor records the operation the ctx carries and then runs the call
// inline. It is the innermost layer the tests chain under the emitting resilience
// wrapper, so what it reads is precisely what the emitter reads.
type captureExecutor struct {
	op    observability.Operation
	hasOp bool
}

func (c *captureExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	c.op, c.hasOp = observability.OperationFrom(ctx)
	return fn(ctx)
}
func (c *captureExecutor) Close() error { return nil }

// instrumentedClient returns a fake client whose guard is the real resilience
// wrapper over a capturing inner — the composition the wiring builds — plus the
// capturing inner so a test can read the declared operation.
func instrumentedClient(t *testing.T) (*fakeMQTTClient, *captureExecutor) {
	t.Helper()
	cl := &fakeMQTTClient{}
	cap := &captureExecutor{}
	clientGuards.Store(cl, &clientGuard{
		exec:         observability.WrapClientExecutor(cap, "mqtt", "mqtt:test"),
		serviceLabel: "mqtt:test",
	})
	t.Cleanup(func() { closeResilience(cl) })
	return cl, cap
}

// GuardedPublish must declare the publish identity before the executor reads it,
// so the emitted span is named "publish" and reports under messaging.client.*.
func TestGuardedPublishDeclaresPublishOperation(t *testing.T) {
	cl, cap := instrumentedClient(t)

	err := GuardedPublish(context.Background(), cl, "sensors/temp", 1, false, []byte("21.5"))
	assert.Error(t, err).Nil()

	assert.That(t, cap.hasOp).True()
	assert.That(t, cap.op.Name).Equal("publish")
	assert.That(t, cap.op.Metric).Equal("messaging.client")
	assert.That(t, cap.op.LogTag).Equal(accessTag)
	// The publish actually ran through the guarded call site.
	assert.That(t, cl.published.Load()).Equal(int32(1))
}

// GuardedConsume declares the consume identity and runs the handler under the
// executor, so a raw-client subscription is observed the same way a publish is.
func TestGuardedConsumeDeclaresConsumeOperation(t *testing.T) {
	cl, cap := instrumentedClient(t)

	ran := false
	err := GuardedConsume(context.Background(), cl, fakeMessage("sensors/temp"), func(context.Context) error {
		ran = true
		return nil
	})
	assert.Error(t, err).Nil()

	assert.That(t, ran).True()
	assert.That(t, cap.hasOp).True()
	assert.That(t, cap.op.Name).Equal("consume")
	assert.That(t, cap.op.Metric).Equal("messaging.client")
}

// With no executor attached (governance off) both entries still declare the
// operation and run the call inline — a standalone, non-gs caller's behaviour.
func TestGuardedEntriesRunInlineWithoutExecutor(t *testing.T) {
	cl := &fakeMQTTClient{} // no guard registered

	rerr := GuardedPublish(context.Background(), cl, "sensors/temp", 1, false, []byte("x"))
	assert.Error(t, rerr).Nil()
	assert.That(t, cl.published.Load()).Equal(int32(1))

	ran := false
	rerr = GuardedConsume(context.Background(), cl, fakeMessage("sensors/temp"), func(context.Context) error {
		ran = true
		return nil
	})
	assert.Error(t, rerr).Nil()
	assert.That(t, ran).True()
}

// fakeMessage is a minimal mqtt.Message carrying only the topic.
type fakeMessage string

func (m fakeMessage) Duplicate() bool   { return false }
func (m fakeMessage) Qos() byte         { return 0 }
func (m fakeMessage) Retained() bool    { return false }
func (m fakeMessage) Topic() string     { return string(m) }
func (m fakeMessage) MessageID() uint16 { return 0 }
func (m fakeMessage) Payload() []byte   { return nil }
func (m fakeMessage) Ack()              {}
