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

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/log"
	"go-spring.org/spring/cloud/messaging"
)

// defaultQoS is the MQTT quality-of-service level the binder publishes and
// subscribes with. Level 1 (at-least-once) gives reliable delivery without the
// overhead of the QoS 2 handshake; use the raw mqtt.Client bean when a topic
// needs a different level.
const defaultQoS byte = 1

// NewBinder adapts an MQTT client to the broker-neutral messaging.Binder, so
// application code can publish/consume messaging.Message envelopes without
// depending on the paho API. destination/source strings are MQTT topics; the
// subscriber group is unused because MQTT 3.1.1 has no shared-subscription
// concept (every subscriber to a topic receives every message). The raw
// mqtt.Client bean stays available for retained messages, custom QoS, wildcards
// and other MQTT features this binder does not model.
//
// Envelope mapping is payload-only: MQTT 3.1.1 packets carry no per-message
// metadata, so Key, Headers and Timestamp are NOT transmitted. That also means
// W3C trace context cannot ride the message, so this binder emits no producer /
// consumer spans (unlike the Kafka/NATS/Pulsar/RabbitMQ binders). Applications
// needing message metadata should use an MQTT 5.0 broker/client or a different
// transport.
func NewBinder(cl mqtt.Client) messaging.Binder {
	return &binder{cl: cl}
}

type binder struct{ cl mqtt.Client }

func (b *binder) NewPublisher(_ context.Context, destination string) (messaging.Publisher, error) {
	return &publisher{cl: b.cl, topic: destination}, nil
}

func (b *binder) NewSubscriber(_ context.Context, source, _ string) (messaging.Subscriber, error) {
	return &subscriber{cl: b.cl, topic: source}, nil
}

// publisher sends the envelope payload to a fixed topic at defaultQoS.
type publisher struct {
	cl    mqtt.Client
	topic string
}

func (p *publisher) Publish(_ context.Context, msg *messaging.Message) error {
	token := p.cl.Publish(p.topic, defaultQoS, false, msg.Payload)
	token.Wait()
	return token.Error()
}

func (p *publisher) Close() error { return nil }

// subscriber delivers messages from a fixed topic to a handler via paho's
// callback. Because the callback is fire-and-forget there is no ack/nack; a
// handler error is logged.
type subscriber struct {
	cl    mqtt.Client
	topic string
}

func (s *subscriber) Subscribe(_ context.Context, handler messaging.Handler) error {
	token := s.cl.Subscribe(s.topic, defaultQoS, func(_ mqtt.Client, m mqtt.Message) {
		msg := &messaging.Message{Payload: m.Payload()}
		if err := handler(context.Background(), msg); err != nil {
			log.Errorf(context.Background(), log.TagAppDef, "mqtt binder handler error on %q: %v", m.Topic(), err)
		}
	})
	token.Wait()
	return token.Error()
}

func (s *subscriber) Close() error {
	token := s.cl.Unsubscribe(s.topic)
	token.Wait()
	return token.Error()
}
