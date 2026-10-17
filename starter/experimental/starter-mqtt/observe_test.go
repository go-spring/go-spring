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
	"go-spring.org/cloud/runtime/dataplane/chain"
	"go.opentelemetry.io/otel/attribute"
	"strings"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// A connection event's counter attribute and its log field must be the same key
// with the same value, or a dashboard selecting
// messaging.client.connection.state_changes{state=...} lands on lines that
// cannot be joined to it. record returns both together for exactly that
// reason; this pins the field side.
func TestConnStateRecordCarriesTheMetricKeys(t *testing.T) {
	fields := newConnStateCounter().record(context.Background(), "probe")

	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	assert.That(t, keys).Equal([]string{"messaging.system", "state"})
}

// operation is the starter's whole declaration: the bounded attributes become
// metric labels, so they must carry only the family vocabulary; the topic rides
// in Detail (span + log only), and the access tag rides along.
func TestOperationDeclaresBoundedAttrsAndTopicDetail(t *testing.T) {
	op := operation(opSend, "sensors/temp")

	assert.That(t, op.Request.Action).Equal("send sensors/temp")
	assert.That(t, op.Observe.Metric.Name).Equal("messaging.client")
	assert.That(t, op.Observe.Log.Tag).Equal(accessTag)

	keys := make([]string, 0, len(op.Observe.Metric.Attrs))
	for _, a := range op.Observe.Metric.Attrs {
		keys = append(keys, string(a.Key))
	}
	assert.That(t, keys).Equal([]string{"messaging.system", "messaging.operation"})

	assert.That(t, len(detailAttrs(op))).Equal(1)
	assert.That(t, string(detailAttrs(op)[0].Key)).Equal("messaging.destination.name")
	assert.That(t, detailAttrs(op)[0].Value.AsString()).Equal("sensors/temp")
}

// A topicless call carries no detail at all, which is also what levelled its
// success log at Info rather than Debug.
func TestOperationOmitsDetailWithoutTopic(t *testing.T) {
	op := operation(opProcess, "")
	assert.That(t, len(detailAttrs(op))).Equal(0)
}

// The topic in Detail is bounded — a topic can be arbitrarily long and a span or
// a log line has no use for all of it.
func TestOperationTruncatesTopic(t *testing.T) {
	op := operation(opSend, strings.Repeat("a", 512+64))
	assert.That(t, len(detailAttrs(op)[0].Value.AsString())).Equal(512)
}

// detailAttrs returns the operation's span attributes that are NOT metric
// labels — the per-call detail the old Call.Detail field held, reconstructed
// from the split destination lists.
func detailAttrs(op chain.Operation) []attribute.KeyValue {
	labeled := make(map[attribute.Key]bool, len(op.Observe.Metric.Attrs))
	for _, a := range op.Observe.Metric.Attrs {
		labeled[a.Key] = true
	}
	var out []attribute.KeyValue
	for _, a := range op.Observe.Span.Attrs {
		if !labeled[a.Key] {
			out = append(out, a)
		}
	}
	return out
}
