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

package StarterGoRedis

import (
	"context"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
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

// pipeCmd builds the command under test the same way go-redis builds a live one.
func pipeCmd(name string, args ...interface{}) redis.Cmder {
	return redis.NewStringCmd(context.Background(), append([]interface{}{name}, args...)...)
}

// TestOperationDeclaresFamilyVocabulary proves one command declares the
// identity the emitter needs: the command's full name for the span, the
// family's metric prefix, the db.* labels and this starter's access-log tag.
func TestOperationDeclaresFamilyVocabulary(t *testing.T) {
	op := operation(pipeCmd("get", "user:42"))
	assert.String(t, op.Name).Equal("get")
	assert.String(t, op.Metric).Equal("db.client")
	assert.String(t, attrsLine(op.Attrs)).Equal("db.system=redis,db.operation=get")
	assert.That(t, op.LogTag == accessTag).True()
}

// TestOperationKeyStaysOutOfLabels is the cardinality guard: a key must reach
// the span and the log but never a metric label, or the series multiply without
// bound. It is asserted on Attrs because Attrs is what labels are built from.
func TestOperationKeyStaysOutOfLabels(t *testing.T) {
	op := operation(pipeCmd("get", "user:42"))
	for _, a := range op.Attrs {
		assert.That(t, string(a.Key) == "db.statement").False()
	}
	assert.String(t, attrsLine(op.Detail)).Equal("db.statement=user:42")
}

// TestOperationWithoutArgCarriesNoDetail proves an argument-less command (PING)
// carries no detail, which is also what levelled its success log at Info rather
// than Debug — the same rule the emitter applies.
func TestOperationWithoutArgCarriesNoDetail(t *testing.T) {
	assert.Number(t, len(operation(pipeCmd("ping")).Detail)).Equal(0)
}

// TestOperationTruncatesStatement proves a long key is bounded before it reaches
// the span or the log.
func TestOperationTruncatesStatement(t *testing.T) {
	op := operation(pipeCmd("get", strings.Repeat("k", maxStatement+50)))
	assert.Number(t, len(op.Detail[0].Value.AsString())).Equal(maxStatement)
}

// TestPipelineOperationDeclaresBatch proves a pipeline declares the batch
// identity ("pipeline") with no per-command detail.
func TestPipelineOperationDeclaresBatch(t *testing.T) {
	op := pipelineOperation()
	assert.String(t, op.Name).Equal("pipeline")
	assert.String(t, attrsLine(op.Attrs)).Equal("db.system=redis,db.operation=pipeline")
	assert.Number(t, len(op.Detail)).Equal(0)
}

// TestSkipOpsDeclareNoIdentity proves the health-check PING is a skip op, so it
// declares no Operation and is left to the resilience layer's undeclared path.
func TestSkipOpsDeclareNoIdentity(t *testing.T) {
	_, skip := skipOps[pipeCmd("ping").FullName()]
	assert.That(t, skip).True()
}
