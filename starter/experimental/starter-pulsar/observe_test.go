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
	"strings"
	"testing"

	"go-spring.org/cloud/governance/capability/traffic"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

var testProp = traffic.NewDefaultPropagator(traffic.DefaultBinding())

// attrsLine renders attributes as "k=v" pairs in slice order.
func attrsLine(attrs []attribute.KeyValue) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, string(a.Key)+"="+a.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// TestOperationDeclaresFamilyVocabulary proves one publish/consume declares the
// identity the emitter needs: the direction for the span name, the family's
// metric prefix, the messaging.* labels and this starter's access-log tag.
func TestOperationDeclaresFamilyVocabulary(t *testing.T) {
	op := operation(context.Background(), testProp, opSend, "persistent://public/default/orders")
	assert.String(t, op.Request.Action).Equal("send persistent://public/default/orders")
	assert.String(t, op.Observe.Metric.Name).Equal("messaging.client")
	assert.String(t, attrsLine(op.Observe.Metric.Attrs)).Equal("messaging.system=pulsar,messaging.operation=send")
	assert.That(t, op.Observe.Log.Tag == accessTag).True()
}

// TestOperationTopicStaysOutOfLabels is the cardinality guard: a topic must
// reach the span and the log but never a metric label, or the series multiply
// without bound. It is asserted on Attrs because Attrs is what labels are built
// from.
func TestOperationTopicStaysOutOfLabels(t *testing.T) {
	op := operation(context.Background(), testProp, opProcess, "persistent://public/default/orders")
	for _, a := range op.Observe.Metric.Attrs {
		assert.That(t, string(a.Key) == "messaging.destination.name").False()
	}
	assert.String(t, attrsLine(detailAttrs(op))).Equal("messaging.destination.name=persistent://public/default/orders")
}

// TestOperationWithoutTopicCarriesNoDetail proves a topicless call carries no
// detail, which is also what levelled its success log at Info rather than Debug
// — the same rule the emitter applies.
func TestOperationWithoutTopicCarriesNoDetail(t *testing.T) {
	assert.Number(t, len(detailAttrs(operation(context.Background(), testProp, opProcess, "")))).Equal(0)
}

// TestOperationTruncatesTopic proves a long topic is bounded before it reaches
// the span or the log.
func TestOperationTruncatesTopic(t *testing.T) {
	const maxDetail = 512
	op := operation(context.Background(), testProp, opSend, strings.Repeat("t", maxDetail+50))
	assert.Number(t, len(detailAttrs(op)[0].Value.AsString())).Equal(maxDetail)
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
