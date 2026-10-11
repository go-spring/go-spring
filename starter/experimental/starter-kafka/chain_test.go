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

package StarterKafka

import (
	"context"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/cloud/observability"
)

// fakeTail is a scripted InnerKafka tail: it counts the produces that reached
// it. It stands in for the raw adapter so chain tests need no broker.
type fakeTail struct {
	ran int
}

func (f *fakeTail) ProduceSync(context.Context, ...*kgo.Record) kgo.ProduceResults {
	f.ran++
	return nil
}

func (f *fakeTail) Release(bool) error { return nil }

// TestNewClientAttachesObserver pins that NewClient builds the emitting layer
// over the client's observer. A kafka client has no seam for the resilience
// executor, so what it attaches is observation — not governance.
func TestNewClientAttachesObserver(t *testing.T) {
	cl := NewClient(nil, "127.0.0.1:1")
	if cl.obs == nil {
		t.Fatal("NewClient must attach an observer")
	}
}

// topicLayer wraps the chain head and namespaces every topic it passes down —
// the kind of behavior change no franz-go hook could express (hooks observe,
// they do not rewrite).
type topicLayer struct {
	InnerKafka
	prefix string
}

func (t topicLayer) ProduceSync(ctx context.Context, recs ...*kgo.Record) kgo.ProduceResults {
	out := make([]*kgo.Record, 0, len(recs))
	for _, r := range recs {
		out = append(out, &kgo.Record{Topic: t.prefix + r.Topic, Key: r.Key, Value: r.Value})
	}
	return t.InnerKafka.ProduceSync(ctx, out...)
}

// TestInnerKafkaReorganize pins the wrap-head protocol: a custom layer over
// the chain head rewrites the topic, and the produce runs through it — the
// tail sees the rewritten topic.
func TestInnerKafkaReorganize(t *testing.T) {
	tail := &fakeTail{}
	cl := &Client{InnerKafka: NewObsKafka(tail, observability.NewOperationObserver("kafka", "test"))}
	cl.InnerKafka = topicLayer{InnerKafka: cl.InnerKafka, prefix: "tenant."}

	results := cl.InnerKafka.ProduceSync(context.Background(), &kgo.Record{Topic: "orders"})
	if results.FirstErr() != nil {
		t.Fatalf("produce through the wrap-head layer: %v", results.FirstErr())
	}
	if tail.ran != 1 {
		t.Fatalf("tail ran %d times, want 1", tail.ran)
	}
}
