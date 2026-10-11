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

	"go-spring.org/stdlib/testing/assert"
)

// The guarded entries emit through the client's observer, which produces the
// span, metrics and access log from the declared operation. With no observer
// attached (a standalone client) the calls run inline. These tests drive both
// stances without a live broker — the fake client's Publish is a stub.

// TestGuardedPublishRunsInline proves both guarded entries still run when no
// observer is attached (a standalone, non-container client): the publish reaches
// paho and the consume handler runs.
func TestGuardedPublishRunsInline(t *testing.T) {
	cl := &fakeMQTTClient{} // no observer registered

	assert.Error(t, GuardedPublish(context.Background(), cl, "sensors/temp", 1, false, []byte("21.5"))).Nil()
	assert.That(t, cl.published.Load()).Equal(int32(1))

	ran := false
	err := GuardedConsume(context.Background(), cl, fakeMessage("sensors/temp"), func(context.Context) error {
		ran = true
		return nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, ran).True()
}

// TestOperationVocabulary pins what a publish and a consume declare: the span
// name, the family metric prefix and the access tag. The observer emits from
// this operation, so this is the contract.
func TestOperationVocabulary(t *testing.T) {
	pub := operation(opPublish, "sensors/temp")
	assert.That(t, pub.Name).Equal("publish")
	assert.That(t, pub.Metric).Equal("messaging.client")
	assert.That(t, pub.LogTag).Equal(accessTag)

	con := operation(opConsume, "sensors/temp")
	assert.That(t, con.Name).Equal("consume")
	assert.That(t, con.Metric).Equal("messaging.client")
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
