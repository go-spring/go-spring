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

package StarterMilvus

import (
	"context"
	"strings"
	"testing"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
)

// attrsLine renders attributes as "k=v" pairs in slice order.
func attrsLine(attrs []attribute.KeyValue) string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		parts = append(parts, string(a.Key)+"="+a.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// TestOperationDeclaresFamilyVocabulary proves one RPC declares the identity
// the emitter needs: the operation name for the span, the family's metric
// prefix, the db.* labels and this starter's access-log tag.
func TestOperationDeclaresFamilyVocabulary(t *testing.T) {
	op := operation("/milvus.proto.milvus.MilvusService/Search")
	assert.String(t, op.Name).Equal("Search")
	assert.String(t, op.Metric).Equal("db.client")
	assert.String(t, attrsLine(op.Attrs)).Equal("db.system=milvus,db.operation=Search")
	assert.That(t, op.LogTag == accessTag).True()
}

// TestOperationMethodStaysOutOfLabels is the cardinality guard: the full method
// path must reach the span and the log but never a metric label. It is asserted
// on Attrs because Attrs is what labels are built from.
func TestOperationMethodStaysOutOfLabels(t *testing.T) {
	op := operation("/milvus.proto.milvus.MilvusService/Search")
	for _, a := range op.Attrs {
		assert.That(t, string(a.Key) == "db.method").False()
	}
	assert.String(t, attrsLine(op.Detail)).Equal("db.method=/milvus.proto.milvus.MilvusService/Search")
}

// TestOperationTakesBoundedMethodName proves the operation name is only the
// bounded last segment — the finest dimension the gRPC interceptor can see.
func TestOperationTakesBoundedMethodName(t *testing.T) {
	op := operation("/milvus.proto.milvus.MilvusService/CreateCollection")
	assert.String(t, op.Name).Equal("CreateCollection")
	assert.That(t, strings.Contains(attrsLine(op.Attrs), "db.operation=CreateCollection")).True()
}

// TestOperationWithoutSlashKeepsWholeWord proves a bare method string (no path
// separator) still declares a name and a detail rather than an empty one.
func TestOperationWithoutSlashKeepsWholeWord(t *testing.T) {
	op := operation("Flush")
	assert.String(t, op.Name).Equal("Flush")
	assert.String(t, attrsLine(op.Detail)).Equal("db.method=Flush")
}

// captureExecutor is a chain.Executor that runs fn with the ctx it was handed,
// so the test sees exactly the ctx the emitter reads the operation from at
// Execute entry.
type captureExecutor struct {
	seen observation
}

type observation struct {
	op observability.Operation
	ok bool
}

func (e *captureExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	e.seen.op, e.seen.ok = observability.OperationFrom(ctx)
	return fn(ctx)
}
func (e *captureExecutor) Close() error { return nil }

// TestUnaryGuardDeclaresOperationBeforeExecuting pins the wire-up: the
// interceptor must put the declaration on the context the executor reads it
// from BEFORE exec.Execute runs. A declaration made inside the executor's fn
// would be read by nobody; here the executor's own entry already sees it.
func TestUnaryGuardDeclaresOperationBeforeExecuting(t *testing.T) {
	exec := &captureExecutor{}
	slot := &guardSlot{}
	slot.apply(exec)

	var ran int
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		ran++
		return nil
	}
	err := unaryGuard(slot)(context.Background(), "/milvus.proto.milvus.MilvusService/Insert", nil, nil, nil, invoker)
	assert.Error(t, err).Nil()
	assert.That(t, ran).Equal(1)
	assert.That(t, exec.seen.ok).True()
	assert.String(t, exec.seen.op.Metric).Equal("db.client")
	assert.String(t, exec.seen.op.Name).Equal("Insert")
}

// TestStreamGuardDeclaresOperationBeforeExecuting does the same for the stream
// interceptor's open.
func TestStreamGuardDeclaresOperationBeforeExecuting(t *testing.T) {
	exec := &captureExecutor{}
	slot := &guardSlot{}
	slot.apply(exec)

	streamer := func(context.Context, *grpc.StreamDesc, *grpc.ClientConn, string, ...grpc.CallOption) (grpc.ClientStream, error) {
		return nil, nil
	}
	_, err := streamGuard(slot)(context.Background(), &grpc.StreamDesc{}, nil, "/milvus.proto.milvus.MilvusService/Search", streamer)
	assert.Error(t, err).Nil()
	assert.That(t, exec.seen.ok).True()
	assert.String(t, exec.seen.op.Name).Equal("Search")
}
