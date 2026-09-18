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

package log

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

var ctxTestTag = RegisterTag("_ctx_test")

// keyOrder renders the field keys in slice order, which is the order
// [contextFields] promises and therefore the order a duplicate key resolves in
// (later wins).
func keyOrder(fields []Field) string {
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	return strings.Join(keys, ",")
}

// captureOutput runs fn with the console stream redirected and returns what was
// written, restoring the stream afterwards.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	prev := Stdout
	buf := bytes.NewBuffer(nil)
	Stdout = buf
	defer func() { Stdout = prev }()
	fn()
	return buf.String()
}

// TestWithFieldsAccumulates proves fields accumulate down the derivation chain
// rather than replacing one another, so several sources can contribute.
func TestWithFieldsAccumulates(t *testing.T) {
	ctx := context.Background()
	ctx = WithFields(ctx, String("user", "u1"))
	ctx = WithFields(ctx, String("tenant", "t1"))
	assert.String(t, keyOrder(contextFields(ctx))).Equal("user,tenant")
}

// TestWithFieldsLeavesSiblingsAlone proves a derivation does not mutate the
// context it came from: two branches off one parent stay independent.
func TestWithFieldsLeavesSiblingsAlone(t *testing.T) {
	parent := WithFields(context.Background(), String("user", "u1"))
	_ = WithFields(parent, String("branch", "a"))

	assert.String(t, keyOrder(contextFields(parent))).Equal("user")
}

// TestWithFieldsDuplicateKeyKeepsEvaluationOrder pins the tie-break rule: both
// copies survive in order, so the later one wins wherever duplicates collapse.
func TestWithFieldsDuplicateKeyKeepsEvaluationOrder(t *testing.T) {
	ctx := context.Background()
	ctx = WithFields(ctx, String("k", "outer"))
	ctx = WithFields(ctx, String("k", "inner"))
	assert.String(t, keyOrder(contextFields(ctx))).Equal("k,k")
}

// TestNoFieldsIsTheSameContext proves WithFields with nothing to add does not
// wrap the context, so the common "may or may not have fields" call site is
// free.
func TestNoFieldsIsTheSameContext(t *testing.T) {
	ctx := context.Background()
	if WithFields(ctx) != ctx {
		t.Fatal("WithFields with no fields should return the context unchanged")
	}
}

// TestCollectWithoutCollectorIsNoOp proves accumulating outside an installed
// Collector is silently ignored rather than panicking: instrumentation must not
// break a request.
func TestCollectWithoutCollectorIsNoOp(t *testing.T) {
	Collect(context.Background(), String("user", "u1"))
	Collect(nil, String("user", "u1"))
	assert.String(t, keyOrder(contextFields(context.Background()))).Equal("")
}

// TestCollectorFieldsSnapshot proves Fields returns a copy: a reader holding
// the result is unaffected by later accumulation.
func TestCollectorFieldsSnapshot(t *testing.T) {
	ctx, c := NewCollector(context.Background())
	Collect(ctx, String("step", "one"))

	snapshot := c.Fields()
	Collect(ctx, String("step", "two"))

	assert.String(t, keyOrder(snapshot)).Equal("step")
	assert.String(t, keyOrder(c.Fields())).Equal("step,step")
}

// TestContextFieldsOrderCarrierThenCollector pins the order the two channels
// merge in: the WithFields chain first, the Collector's accumulated fields
// after.
func TestContextFieldsOrderCarrierThenCollector(t *testing.T) {
	ctx, _ := NewCollector(context.Background())
	ctx = WithFields(ctx, String("user", "u1"))
	Collect(ctx, String("order", "o1"))

	assert.String(t, keyOrder(contextFields(ctx))).Equal("user,order")
}

// TestContextFieldsReachLogOutput is the end-to-end contract: a field put on a
// context shows up on a log event printed with that context, with no call site
// opting in -- which is what lets framework-emitted lines carry them too.
func TestContextFieldsReachLogOutput(t *testing.T) {
	ctx := context.Background()
	ctx = WithFields(ctx, String("user", "u1"))

	out := captureOutput(t, func() {
		Info(ctx, ctxTestTag, Msgf("hello"))
	})
	assert.String(t, out).Contains("user=u1")
}

// TestCollectedFieldsReachLogOutput proves the Collector channel is wired into
// the same path, so a wide-event line printed at the end sees what handlers
// accumulated during the request.
func TestCollectedFieldsReachLogOutput(t *testing.T) {
	ctx, _ := NewCollector(context.Background())
	Collect(ctx, String("order", "o1"))

	out := captureOutput(t, func() {
		Info(ctx, ctxTestTag, Msgf("hello"))
	})
	assert.String(t, out).Contains("order=o1")
}

// TestFieldsFromContextHookKeepsLastWord proves a user-installed hook still
// overrides what the context carries -- it is appended last, so it cannot be
// silently shadowed by the new channels. This is what keeps the luohua
// composition hook working unchanged.
func TestFieldsFromContextHookKeepsLastWord(t *testing.T) {
	prev := FieldsFromContext
	defer func() { FieldsFromContext = prev }()
	FieldsFromContext = func(ctx context.Context) []Field {
		return []Field{String("user", "from-hook")}
	}

	ctx := WithFields(context.Background(), String("user", "from-carrier"))

	out := captureOutput(t, func() {
		Info(ctx, ctxTestTag, Msgf("hello"))
	})
	assert.String(t, out).Contains("user=from-carrier")
	assert.String(t, out).Contains("user=from-hook")
	// The hook's value is written last, so it is the one a text reader sees.
	if strings.LastIndex(out, "from-hook") < strings.LastIndex(out, "from-carrier") {
		t.Fatalf("the hook's field should be written after the carrier's, got: %s", out)
	}
}
