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
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/stdlib/testing/assert"
)

// TestTopicOfNamesOnlyASharedDestination pins the batch contract: one call is
// one operation with ONE destination or with none. A mixed batch declares no
// destination, because the value reaches the span NAME and the governance rule
// key — naming such a batch after its first record would attribute it to a topic
// only part of it was produced to, and a rule aimed at that topic would match it.
func TestTopicOfNamesOnlyASharedDestination(t *testing.T) {
	rec := func(topic string) *kgo.Record { return &kgo.Record{Topic: topic} }
	recs := func(topics ...string) []*kgo.Record {
		out := make([]*kgo.Record, 0, len(topics))
		for _, topic := range topics {
			out = append(out, rec(topic))
		}
		return out
	}

	assert.String(t, topicOf(nil)).Equal("")
	assert.String(t, topicOf(recs("orders"))).Equal("orders")
	assert.String(t, topicOf(recs("orders", "orders"))).Equal("orders")
	assert.String(t, topicOf(recs("orders", "payments"))).Equal("")
	assert.String(t, topicOf(recs("orders", "orders", "payments"))).Equal("")
}
