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

package resilience

import (
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// listenerRecorder is a provider-built executor that records whether [resolve]
// handed it a breaker listener. It stands in for the default driver, whose
// breakers can only route state transitions to a listener captured at
// construction.
type listenerRecorder struct {
	fakeExecutor
	listener BreakerEventListener
}

func (e *listenerRecorder) SetBreakerEventListener(l BreakerEventListener) { e.listener = l }

// TestExecutorForAttachesListenerThroughResolve locks in the ordering that
// [resolve] depends on: the observe layer's SetBreakerEventListener handshake
// must reach the provider-built executor BEFORE it is published, so its
// per-resource breakers — built lazily on first Execute — capture the listener.
//
// Regression: clients used to wrap from outside a resolvedExecutor/faultExecutor
// chain, so the type assertion in WrapExecutor never matched the real executor
// and the listener stayed nil — breaker.state_change silently emitted nothing.
func TestExecutorForAttachesListenerThroughResolve(t *testing.T) {
	built := &listenerRecorder{}
	p := Provider(func(string) Executor { return built })
	provider.Store(&p)
	cache.Delete("test:attach")
	t.Cleanup(func() {
		cache.Delete("test:attach")
		provider.Store(nil)
	})

	exec := ExecutorFor("test", "test:attach")
	err := exec.Execute(t.Context(), "", func(context.Context) error { return nil })
	assert.That(t, err).Nil()
	assert.That(t, built.listener != nil).True()
}
