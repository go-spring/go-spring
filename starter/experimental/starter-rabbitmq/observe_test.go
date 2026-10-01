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
	fields := newConnStateCounter().record(context.Background(), connBlocked)

	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	assert.That(t, keys).Equal([]string{"messaging.system", "state"})
}

// The declared operation is the whole contract this starter hands the emitter:
// bounded labels in Attrs, the unbounded destination in Detail, and never the
// other way around. A destination in Attrs would become a metric label and
// multiply the series without bound — the rule this pins.
func TestOperationDeclaresBoundedAttrsAndUnboundedDetail(t *testing.T) {
	op := operation(opPublish, "orders.created")

	assert.That(t, op.Name).Equal("publish")
	assert.That(t, op.Metric).Equal("messaging.client")
	if op.LogTag == nil {
		t.Fatal("a declared operation must carry the module's access tag")
	}
	keys := make([]string, 0, len(op.Attrs))
	for _, a := range op.Attrs {
		keys = append(keys, string(a.Key))
	}
	assert.That(t, keys).Equal([]string{"messaging.system", "messaging.operation"})
	if len(op.Detail) != 1 || string(op.Detail[0].Key) != "messaging.destination.name" {
		t.Fatalf("destination must ride in Detail, got %v", op.Detail)
	}
}

// A destinationless call carries no Detail at all — which is what levelled its
// success log at Info rather than Debug.
func TestOperationWithoutDestinationHasNoDetail(t *testing.T) {
	if op := operation(opConsume, ""); len(op.Detail) != 0 {
		t.Fatalf("a destinationless operation must carry no detail, got %v", op.Detail)
	}
}

// The destination is bounded at the same limit the access log applied.
func TestOperationTruncatesDestination(t *testing.T) {
	op := operation(opPublish, strings.Repeat("q", maxDestination+100))
	if got := len(op.Detail[0].Value.AsString()); got != maxDestination {
		t.Fatalf("destination must be truncated to %d, got %d", maxDestination, got)
	}
}
