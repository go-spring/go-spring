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

// TestOperationDeclaresFamilyVocabulary proves one operation declares the
// identity the emitter needs: the operation name for the span, the family's
// metric prefix, the db.* labels and this starter's access-log tag.
func TestOperationDeclaresFamilyVocabulary(t *testing.T) {
	op := operation("query", "MATCH (n) RETURN n")
	assert.String(t, op.Name).Equal("query")
	assert.String(t, op.Metric).Equal("db.client")
	assert.String(t, attrsLine(op.Attrs)).Equal("db.system=neo4j,db.operation=query")
	assert.That(t, op.LogTag == accessTag).True()
}

// TestOperationStatementStaysOutOfLabels is the cardinality guard: a Cypher
// statement must reach the span and the log but never a metric label, or the
// series multiply without bound. It is asserted on Attrs because Attrs is what
// labels are built from.
func TestOperationStatementStaysOutOfLabels(t *testing.T) {
	op := operation("query", "MATCH (n) RETURN n")
	for _, a := range op.Attrs {
		assert.That(t, string(a.Key) == "db.statement").False()
	}
	assert.String(t, attrsLine(op.Detail)).Equal("db.statement=MATCH (n) RETURN n")
}

// TestOperationWithoutStatementCarriesNoDetail proves an operation with no
// Cypher carries no detail, which is also what levelled its success log at Info
// rather than Debug — the same rule the emitter applies.
func TestOperationWithoutStatementCarriesNoDetail(t *testing.T) {
	assert.Number(t, len(operation("query", "").Detail)).Equal(0)
}

// TestOperationTruncatesStatement proves a long Cypher text is bounded before it
// reaches the span or the log.
func TestOperationTruncatesStatement(t *testing.T) {
	op := operation("query", strings.Repeat("k", maxStatement+50))
	assert.Number(t, len(op.Detail[0].Value.AsString())).Equal(maxStatement)
}
