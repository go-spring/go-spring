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
	"testing"

	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/traffic"
)

// fakePulsarClient is a map-key-only stand-in for the client bean.
type fakePulsarClient struct{ pulsar.Client }

// okPulsarProducer satisfies the producer surface the publisher touches:
// Topic() before the send, Send() inside it.
type okPulsarProducer struct {
	pulsar.Producer
	topic string
	ran   int
}

func (p *okPulsarProducer) Topic() string { return p.topic }

func (p *okPulsarProducer) Send(context.Context, *pulsar.ProducerMessage) (pulsar.MessageID, error) {
	p.ran++
	return nil, nil
}

// TestAttachObserverAttaches pins that AttachObserver indexes an observer for the
// client — the driver calls it while building the client, so a client is never
// left unobserved — and that forgetObserver drops it again.
func TestAttachObserverAttaches(t *testing.T) {
	cl := &fakePulsarClient{}
	AttachObserver(cl, "pulsar://127.0.0.1:6650")
	if _, ok := clientGuards.Load(cl); !ok {
		t.Fatal("AttachObserver must attach an observer")
	}
	forgetObserver(cl)
	if _, ok := clientGuards.Load(cl); ok {
		t.Fatal("forgetObserver must drop the observer")
	}
}

// TestDriverPublishRunsInlineWithoutObserver proves a client with no observer
// attached (a hand-built one) still publishes: the send runs inline.
func TestDriverPublishRunsInlineWithoutObserver(t *testing.T) {
	cl := &fakePulsarClient{}
	prod := &okPulsarProducer{topic: "t"}
	prop := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	p := &publisher{cl: cl, p: prod, prop: prop}
	if err := p.Publish(context.Background(), &messaging.Message{Payload: []byte("x")}); err != nil {
		t.Fatalf("publish without an observer must run inline: %v", err)
	}
	if prod.ran != 1 {
		t.Fatalf("send ran %d times, want 1", prod.ran)
	}
}
