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

package transaction

import (
	"context"
	"sync"

	"go-spring.org/spring/aspect"
)

// sagaIDKey is the context key carrying a saga id set by [WithSagaID].
type sagaIDKey struct{}

// WithSagaID returns a context that carries id, so [GlobalTransactional] can pick
// up the saga's idempotency key from the request context instead of the business
// method inventing one. Set it at the edge (a middleware deriving it from a
// request/idempotency header).
func WithSagaID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sagaIDKey{}, id)
}

// SagaIDFromContext returns the saga id previously stored by [WithSagaID], and
// whether one was present.
func SagaIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(sagaIDKey{}).(string)
	return id, ok
}

// StepRegistry maps a logical method name to the ordered [Step]s that make up
// its Saga. It is the explicit, no-reflection equivalent of Java's
// @GlobalTransactional declaration: business code registers a method's steps
// once at wiring time, and [GlobalTransactional] looks them up by the
// joinpoint's method name. It is safe for concurrent use.
type StepRegistry struct {
	mu    sync.RWMutex
	steps map[string][]Step
}

// NewStepRegistry returns an empty registry.
func NewStepRegistry() *StepRegistry {
	return &StepRegistry{steps: make(map[string][]Step)}
}

// Register associates method with steps, replacing any previous registration for
// the same method. steps is copied so later caller mutations do not alias the
// registry.
func (r *StepRegistry) Register(method string, steps ...Step) {
	cp := append([]Step(nil), steps...)
	r.mu.Lock()
	if r.steps == nil {
		r.steps = make(map[string][]Step)
	}
	r.steps[method] = cp
	r.mu.Unlock()
}

// Lookup returns the steps registered for method and whether any were found.
func (r *StepRegistry) Lookup(method string) ([]Step, bool) {
	r.mu.RLock()
	steps, ok := r.steps[method]
	r.mu.RUnlock()
	return steps, ok
}

// GlobalTransactional returns an [aspect.Interceptor] that provides the
// @GlobalTransactional equivalent: when the joinpoint's method has steps
// registered in reg, it runs them as a [Saga] through coord instead of proceeding
// to the target; when it does not, it proceeds transparently. This keeps the
// wiring a no-op for un-declared methods and does not parse or reflect over the
// business call.
//
// The saga id is taken from the context ([WithSagaID]); absent that, the method
// name is used as a last resort, which is only correct for a single in-flight
// instance and is why callers are expected to set an explicit id. Combine with
// [aspect.Only] to scope it to specific methods within a shared chain.
func GlobalTransactional(coord Coordinator, reg *StepRegistry) aspect.Interceptor {
	return aspect.InterceptorFunc(func(jp *aspect.Joinpoint) (any, error) {
		steps, ok := reg.Lookup(jp.Method)
		if !ok {
			return jp.Proceed(jp.Context)
		}
		id, ok := SagaIDFromContext(jp.Context)
		if !ok {
			id = jp.Method
		}
		return coord.Execute(jp.Context, Saga{ID: id, Method: jp.Method, Steps: steps})
	})
}
