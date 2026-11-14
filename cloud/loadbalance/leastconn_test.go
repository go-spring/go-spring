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

package loadbalance

import (
	"context"
	"sync"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestLeastConn(t *testing.T) {
	b := NewLeastConn()
	set := eps("a", "b")

	// Two picks without releasing: each must land on a different endpoint since
	// the first one is now at in-flight 1.
	e1, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	e2, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, e1.Addr).NotEqual(e2.Addr)

	// Release e1; it now has the fewest in-flight and must be chosen.
	b.Complete(context.Background(), e1, nil)
	e3, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, e3.Addr).Equal(e1.Addr)

	// A repeated Complete for the same request is a harmless no-op: the count
	// deletes at zero instead of going negative (interface contract).
	b.Complete(context.Background(), e1, nil)
	b.Complete(context.Background(), e1, nil)
	e4, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, e4.Addr).Equal(e1.Addr) // e1's count was not driven negative
}

// TestLeastConnTieRotation pins the tie-breaker: with every count equal, the
// rotating start spreads picks evenly instead of always taking the first
// endpoint.
func TestLeastConnTieRotation(t *testing.T) {
	b := NewLeastConn()
	set := eps("a", "b", "c")

	m := map[string]int{}
	for range 6 {
		ep, err := b.Pick(set, PickInfo{})
		assert.Error(t, err).Nil()
		m[ep.Addr]++
		b.Complete(context.Background(), ep, nil) // back to zero: every pick sees a tie
	}
	assert.Number(t, m["a"]).Equal(2)
	assert.Number(t, m["b"]).Equal(2)
	assert.Number(t, m["c"]).Equal(2)
}

// TestLeastConnConcurrent exercises Pick/Complete racing on the shared table. Run
// with -race; afterwards every settled request must have left the table empty.
func TestLeastConnConcurrent(t *testing.T) {
	b := NewLeastConn()
	set := eps("a", "b", "c")

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				ep, err := b.Pick(set, PickInfo{})
				assert.Error(t, err).Nil()
				b.Complete(context.Background(), ep, nil)
			}
		})
	}
	wg.Wait()

	b.mu.Lock()
	defer b.mu.Unlock()
	assert.Number(t, len(b.inflight)).Equal(0)
}
