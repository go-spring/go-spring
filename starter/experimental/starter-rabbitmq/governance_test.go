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

package StarterRabbitMQ

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/cloud/governance/capability/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// fakeTail is a scripted InnerPublisher tail: it counts the publishes that
// reached it and answers from a settable error. It stands in for the raw
// adapter so chain tests need no live broker.
type fakeTail struct {
	ran int
	err error
}

func (f *fakeTail) Publish(context.Context, *amqp.Channel, string, string, bool, bool, amqp.Publishing) error {
	f.ran++
	return f.err
}

func (f *fakeTail) Release(bool) error { return nil }

// queueLayer wraps the chain head and namespaces every queue name it passes
// down — the kind of behavior change no amqp middleware could express.
type queueLayer struct {
	InnerPublisher
	prefix string
}

func (q queueLayer) Publish(ctx context.Context, ch *amqp.Channel, exchange, key string, mandatory, immediate bool, pub amqp.Publishing) error {
	return q.InnerPublisher.Publish(ctx, ch, exchange, q.prefix+key, mandatory, immediate, pub)
}

// TestInnerPublisherReorganize pins the wrap-head protocol: a custom layer
// over the chain head rewrites the routing key, and the publish runs through
// it — the tail sees the rewritten key.
func TestInnerPublisherReorganize(t *testing.T) {
	tail := &fakeTail{}
	// No observer: the emitting layer runs the publish inline.
	cl := &Client{InnerPublisher: NewObsPublisher(tail, nil, traffic.NewDefaultPropagator(traffic.DefaultBinding()))}
	cl.InnerPublisher = queueLayer{InnerPublisher: cl.InnerPublisher, prefix: "tenant."}

	err := cl.InnerPublisher.Publish(context.Background(), nil, "", "jobs", false, false, amqp.Publishing{})
	if err != nil {
		t.Fatalf("publish through the wrap-head layer: %v", err)
	}
	assert.That(t, tail.ran).Equal(1)
}
