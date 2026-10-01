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
	"strings"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// attrsLine renders attributes as "k=v" pairs in slice order.
func attrsLine(attrs []attribute.KeyValue) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, string(a.Key)+"="+a.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// TestOperationDeclaresFamilyVocabulary proves one direction declares the
// identity the emitter needs: the direction for the span, the family's metric
// prefix, the messaging.* labels and this starter's access-log tag.
func TestOperationDeclaresFamilyVocabulary(t *testing.T) {
	op := operation(opPublish, "hello")
	assert.String(t, op.Name).Equal("publish")
	assert.String(t, op.Metric).Equal("messaging.client")
	assert.String(t, attrsLine(op.Attrs)).Equal("messaging.system=kafka,messaging.operation=publish")
	assert.That(t, op.LogTag == accessTag).True()
}

// TestOperationTopicStaysOutOfLabels is the cardinality guard: a topic must
// reach the span and the log but never a metric label, or the series multiply
// without bound. It is asserted on Attrs because Attrs is what labels are built
// from.
func TestOperationTopicStaysOutOfLabels(t *testing.T) {
	op := operation(opConsume, "hello")
	for _, a := range op.Attrs {
		assert.That(t, string(a.Key) == "messaging.destination.name").False()
	}
	assert.String(t, attrsLine(op.Detail)).Equal("messaging.destination.name=hello")
}

// TestOperationWithoutTopicCarriesNoDetail proves a topicless call carries no
// detail, which is also what levelled its success log at Info rather than Debug
// — the same rule the emitter applies.
func TestOperationWithoutTopicCarriesNoDetail(t *testing.T) {
	assert.Number(t, len(operation(opPublish, "").Detail)).Equal(0)
}

// TestOperationTruncatesTopic proves a long topic is bounded before it reaches
// the span or the log.
func TestOperationTruncatesTopic(t *testing.T) {
	op := operation(opPublish, strings.Repeat("t", maxDestination+50))
	assert.Number(t, len(op.Detail[0].Value.AsString())).Equal(maxDestination)
}
