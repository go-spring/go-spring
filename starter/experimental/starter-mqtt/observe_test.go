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
	op := operation(opPublish, "sensors/temp")

	assert.That(t, op.Name).Equal("publish")
	assert.That(t, op.Metric).Equal("messaging.client")
	assert.That(t, op.LogTag).Equal(accessTag)

	keys := make([]string, 0, len(op.Attrs))
	for _, a := range op.Attrs {
		keys = append(keys, string(a.Key))
	}
	assert.That(t, keys).Equal([]string{"messaging.system", "messaging.operation"})

	assert.That(t, len(op.Detail)).Equal(1)
	assert.That(t, string(op.Detail[0].Key)).Equal("messaging.destination.name")
	assert.That(t, op.Detail[0].Value.AsString()).Equal("sensors/temp")
}

// A topicless call carries no detail at all, which is also what levelled its
// success log at Info rather than Debug.
func TestOperationOmitsDetailWithoutTopic(t *testing.T) {
	op := operation(opConsume, "")
	assert.That(t, len(op.Detail)).Equal(0)
}

// The topic in Detail is bounded — a topic can be arbitrarily long and a span or
// a log line has no use for all of it.
func TestOperationTruncatesTopic(t *testing.T) {
	op := operation(opPublish, strings.Repeat("a", maxTopic+64))
	assert.That(t, len(op.Detail[0].Value.AsString())).Equal(maxTopic)
}
