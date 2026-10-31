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
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
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

// pipeCmd builds the command under test the same way go-redis builds a live one.
func pipeCmd(name string, args ...interface{}) redis.Cmder {
	return redis.NewStringCmd(context.Background(), append([]interface{}{name}, args...)...)
}

// TestOperationDeclaresFamilyVocabulary proves one command declares the
// identity the emitter needs: the command's full name for the span, the
// family's metric prefix, the db.* labels and this starter's access-log tag.
func TestOperationDeclaresFamilyVocabulary(t *testing.T) {
	op := operation(context.Background(), testProp, pipeCmd("get", "user:42"))
	assert.String(t, op.Request.Action).Equal("get")
	assert.String(t, op.Observe.Metric.Name).Equal("db.client")
	assert.String(t, attrsLine(op.Observe.Metric.Attrs)).Equal("db.system=redis,db.operation=get")
	assert.That(t, op.Observe.Log.Tag == accessTag).True()
}

// TestOperationKeyStaysOutOfLabels is the cardinality guard: a key must reach
// the span and the log but never a metric label, or the series multiply without
// bound. It is asserted on Attrs because Attrs is what labels are built from.
func TestOperationKeyStaysOutOfLabels(t *testing.T) {
	op := operation(context.Background(), testProp, pipeCmd("get", "user:42"))
	for _, a := range op.Observe.Metric.Attrs {
		assert.That(t, string(a.Key) == "db.statement").False()
	}
	assert.String(t, attrsLine(detailAttrs(op))).Equal("db.statement=user:42")
}

// TestOperationWithoutArgCarriesNoDetail proves an argument-less command (PING)
// carries no detail, which is also what levelled its success log at Info rather
// than Debug — the same rule the emitter applies.
func TestOperationWithoutArgCarriesNoDetail(t *testing.T) {
	assert.Number(t, len(detailAttrs(operation(context.Background(), testProp, pipeCmd("ping"))))).Equal(0)
}

// TestOperationTruncatesStatement proves a long key is bounded before it reaches
// the span or the log.
func TestOperationTruncatesStatement(t *testing.T) {
	op := operation(context.Background(), testProp, pipeCmd("get", strings.Repeat("k", 512+50)))
	assert.Number(t, len(detailAttrs(op)[0].Value.AsString())).Equal(512)
}

// TestPipelineOperationDeclaresBatch proves a pipeline declares the batch
// identity ("pipeline") with no per-command detail.
func TestPipelineOperationDeclaresBatch(t *testing.T) {
	op := pipelineOperation(context.Background(), testProp)
	assert.String(t, op.Request.Action).Equal("pipeline")
	assert.String(t, attrsLine(op.Observe.Metric.Attrs)).Equal("db.system=redis,db.operation=pipeline")
	assert.Number(t, len(detailAttrs(op))).Equal(0)
}

// TestSkipOpsDeclareNoIdentity proves the health-check PING is a skip op, so it
// declares no Operation and is left to the resilience layer's undeclared path.
func TestSkipOpsDeclareNoIdentity(t *testing.T) {
	_, skip := skipOps[pipeCmd("ping").FullName()]
	assert.That(t, skip).True()
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
