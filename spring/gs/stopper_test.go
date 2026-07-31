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

package gs

import (
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// TestStopperRunsAll proves every registered stopper runs, regardless of
// registration order (stoppers are independent and unordered, like servers).
func TestStopperRunsAll(t *testing.T) {
	resetStoppersForTest()
	t.Cleanup(resetStoppersForTest)

	ran := map[string]bool{}
	RegisterStopper("a", func(context.Context) error { ran["a"] = true; return nil })
	RegisterStopper("b", func(context.Context) error { ran["b"] = true; return nil })
	RegisterStopper("c", func(context.Context) error { ran["c"] = true; return nil })

	runStoppers(context.Background())
	assert.That(t, ran["a"]).True()
	assert.That(t, ran["b"]).True()
	assert.That(t, ran["c"]).True()
}

// TestStopperIdempotent proves a second runStoppers call is a no-op (registry
// drained), so the top-level defer backing up a prior run does not double-flush.
func TestStopperIdempotent(t *testing.T) {
	resetStoppersForTest()
	t.Cleanup(resetStoppersForTest)

	var n int
	RegisterStopper("otel-trace", func(context.Context) error { n++; return nil })

	runStoppers(context.Background())
	assert.Number(t, n).Equal(1)
	runStoppers(context.Background())
	assert.Number(t, n).Equal(1) // already drained, not re-run
}

// TestStopperPanics proves empty name / nil stopper fail loudly at registration.
func TestStopperPanics(t *testing.T) {
	resetStoppersForTest()
	t.Cleanup(resetStoppersForTest)

	assert.Panic(t, func() { RegisterStopper("", func(context.Context) error { return nil }) }, "empty name")
	assert.Panic(t, func() { RegisterStopper("x", nil) }, "nil stopper")
}
