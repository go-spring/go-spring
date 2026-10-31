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

package StarterNeo4j

import (
	"context"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"strings"
	"testing"

	"go-spring.org/cloud/governance/capability/traffic"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// testProp is the process traffic convention these fixtures run under.
var testProp = traffic.NewDefaultPropagator(traffic.DefaultBinding())

// attrsLine renders attributes as "k=v" pairs in slice order.
func attrsLine(attrs []attribute.KeyValue) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, string(a.Key)+"="+a.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// TestOperationDeclaresFamilyVocabulary proves one operation declares the
// identity the emitter needs: the operation name for the span, the family's
// metric prefix, the db.* labels and this starter's access-log tag.
func TestOperationDeclaresFamilyVocabulary(t *testing.T) {
	op := operation(context.Background(), testProp, "query", "MATCH (n) RETURN n")
	assert.String(t, op.Request.Action).Equal("query")
	assert.String(t, op.Observe.Metric.Name).Equal("db.client")
	assert.String(t, attrsLine(op.Observe.Metric.Attrs)).Equal("db.system=neo4j,db.operation=query")
	assert.That(t, op.Observe.Log.Tag == accessTag).True()
}

// TestOperationStatementStaysOutOfLabels is the cardinality guard: a Cypher
// statement must reach the span and the log but never a metric label, or the
// series multiply without bound. It is asserted on Attrs because Attrs is what
// labels are built from.
func TestOperationStatementStaysOutOfLabels(t *testing.T) {
	op := operation(context.Background(), testProp, "query", "MATCH (n) RETURN n")
	for _, a := range op.Observe.Metric.Attrs {
		assert.That(t, string(a.Key) == "db.statement").False()
	}
	assert.String(t, attrsLine(detailAttrs(op))).Equal("db.statement=MATCH (n) RETURN n")
}

// TestOperationWithoutStatementCarriesNoDetail proves an operation with no
// Cypher carries no detail, which is also what levelled its success log at Info
// rather than Debug — the same rule the emitter applies.
func TestOperationWithoutStatementCarriesNoDetail(t *testing.T) {
	assert.Number(t, len(detailAttrs(operation(context.Background(), testProp, "query", "")))).Equal(0)
}

// TestOperationTruncatesStatement proves a long Cypher text is bounded before it
// reaches the span or the log.
func TestOperationTruncatesStatement(t *testing.T) {
	op := operation(context.Background(), testProp, "query", strings.Repeat("k", 512+50))
	assert.Number(t, len(detailAttrs(op)[0].Value.AsString())).Equal(512)
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
