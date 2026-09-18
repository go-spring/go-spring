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

package observability_test

import (
	"context"
	"strings"
	"testing"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// attrsToString renders attributes as "k=v" pairs in slice order, which is the
// order the package promises and therefore the order a duplicate key resolves
// in (later wins). It uses Emit rather than AsString: AsString returns the raw
// string for a STRING value and an empty string for every other type, so a
// non-string assertion written against it would be silently meaningless.
func attrsToString(attrs []attribute.KeyValue) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, string(a.Key)+"="+a.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// TestWithContextAttributesAccumulates proves attributes accumulate down the
// derivation chain rather than replacing one another, so several sources can
// contribute.
func TestWithContextAttributesAccumulates(t *testing.T) {
	ctx := context.Background()
	ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", "t1"))
	ctx = observability.WithContextAttributes(ctx, attribute.String("user", "u1"))
	assert.String(t, attrsToString(observability.ContextAttributes(ctx))).Equal("tenant=t1,user=u1")
}

// TestWithContextAttributesLeavesSiblingsAlone proves a derivation does not
// mutate the context it came from: two branches off one parent stay
// independent.
func TestWithContextAttributesLeavesSiblingsAlone(t *testing.T) {
	parent := observability.WithContextAttributes(context.Background(), attribute.String("tenant", "t1"))
	_ = observability.WithContextAttributes(parent, attribute.String("branch", "b1"))
	assert.String(t, attrsToString(observability.ContextAttributes(parent))).Equal("tenant=t1")
}

// TestWithContextAttributesNoAttrsIsSameContext proves the call is free when
// there is nothing to add, so a call site that may or may not have attributes
// does not pay for a wrapping.
func TestWithContextAttributesNoAttrsIsSameContext(t *testing.T) {
	ctx := context.Background()
	if observability.WithContextAttributes(ctx) != ctx {
		t.Fatal("WithContextAttributes with no attributes should return the context unchanged")
	}
}

// TestWithContextAttributesDuplicateKeyKeepsEvaluationOrder pins the tie-break
// rule: both copies survive in order, so the later one wins wherever duplicates
// collapse.
func TestWithContextAttributesDuplicateKeyKeepsEvaluationOrder(t *testing.T) {
	ctx := context.Background()
	ctx = observability.WithContextAttributes(ctx, attribute.String("k", "outer"))
	ctx = observability.WithContextAttributes(ctx, attribute.String("k", "inner"))
	assert.String(t, attrsToString(observability.ContextAttributes(ctx))).Equal("k=outer,k=inner")
}

// TestContextAttributesOnPlainContextIsNil proves a context that carries
// nothing reports so, which is what lets the reader stay inert.
func TestContextAttributesOnPlainContextIsNil(t *testing.T) {
	assert.String(t, attrsToString(observability.ContextAttributes(context.Background()))).Equal("")
}
