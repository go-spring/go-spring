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

package StarterLockEtcd

import (
	"context"
	"testing"

	"go-spring.org/cloud/lock"
	"go-spring.org/stdlib/testing/assert"
)

// TestObserve_DefaultOn proves the transparent default: with observe enabled
// (the bound-config default) the locker returned by wrapIfObserved is a
// distinct observe-wrapped Locker whose lock semantics still hold, and Close
// passes through to the inner locker.
func TestObserve_DefaultOn(t *testing.T) {
	inner := lock.NewMemoryLocker()
	l := wrapIfObserved(Config{ObserveEnabled: true}, inner)
	if l == lock.Locker(inner) {
		t.Fatal("observe.enabled default must wrap the locker, got the bare locker back")
	}
	held, ok, err := l.TryAcquire(context.Background(), "k", lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	assert.Error(t, held.Unlock(context.Background())).Nil()
	assert.Error(t, l.Close()).Nil()
}

// TestObserve_OptOutBare proves observe.enabled=false returns the bare locker
// unchanged — the opt-out path adds no wrapper.
func TestObserve_OptOutBare(t *testing.T) {
	inner := lock.NewMemoryLocker()
	l := wrapIfObserved(Config{ObserveEnabled: false}, inner)
	if l != lock.Locker(inner) {
		t.Fatal("observe.enabled=false must return the bare locker")
	}
}
