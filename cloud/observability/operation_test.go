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
	"testing"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// TestOperationRoundTrips proves the identity a client declares is the identity
// the emitter reads, attribute values included.
func TestOperationRoundTrips(t *testing.T) {
	op := observability.Operation{
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", "memcached"),
			attribute.String("db.operation", "get"),
		},
	}
	ctx := observability.WithOperation(context.Background(), op)

	got, ok := observability.OperationFrom(ctx)
	assert.That(t, ok).True()
	assert.String(t, got.Metric).Equal("db.client")
	assert.String(t, attrsToString(got.Attrs)).Equal("db.system=memcached,db.operation=get")
}

// TestWithOperationRejectsEmptyMetric proves the guard: the emitter builds its
// instrument names from the prefix, so an empty one would register an invalid
// name whose error is discarded — the client would lose every metric with
// nothing to show for it. A loud failure at the first call is the cheaper bug.
func TestWithOperationRejectsEmptyMetric(t *testing.T) {
	assert.Panic(t, func() {
		observability.WithOperation(context.Background(), observability.Operation{Name: "x"})
	}, "Metric must not be empty")
}

// TestOperationFromPlainContextIsAbsent proves a call with no declared identity
// reports absent rather than the zero Operation masquerading as a real one.
func TestOperationFromPlainContextIsAbsent(t *testing.T) {
	_, ok := observability.OperationFrom(context.Background())
	assert.That(t, ok).False()
}

// TestWithOperationInnermostWins proves a nested declaration replaces the outer
// one: the operation being run is the innermost call's, not the one it is nested
// inside.
func TestWithOperationInnermostWins(t *testing.T) {
	ctx := observability.WithOperation(context.Background(),
		observability.Operation{Metric: "db.client"})
	ctx = observability.WithOperation(ctx,
		observability.Operation{Metric: "messaging.client"})

	got, _ := observability.OperationFrom(ctx)
	assert.String(t, got.Metric).Equal("messaging.client")
}
