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

// Package gsmock provides function-level mocking based on
// runtime function identification and invocation interception.
package gsmock

import (
	"context"
	"fmt"
	"reflect"
)

type managerKeyType struct{}

var managerKey managerKeyType

// WithManager returns a new context with the given Manager attached.
//
// The Manager can later be retrieved by InvokeContext to dispatch mock calls.
// The returned context should typically be passed through the call chain
// where function interception is expected.
func WithManager(ctx context.Context, r *Manager) context.Context {
	return context.WithValue(ctx, &managerKey, r)
}

// Invoker defines the interface that all mock implementations must satisfy.
//
// Invoke receives the original function parameters and returns:
//   - ret: the mocked return values
//   - ok:  whether this Invoker matches the given parameters
//
// The first Invoker that returns ok == true will be used.
type Invoker interface {
	Invoke(params []any) ([]any, bool)
}

// funcKey is the composite key used to index mockers.
//
// fnPC identifies the function or method expression by its program counter (PC).
// receiver is only used when mocking interface methods via generated code.
//
// receiver must be a pointer to a concrete struct type.
// It is non-nil only in interface mocking scenarios.
type funcKey struct {
	receiver any
	fnPC     uintptr
}

// newFuncKey creates a funcKey from a receiver and a function.
// Passing a non-function value will cause a panic.
func newFuncKey(receiver any, fn any) funcKey {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		panic("mock target must be a function or method expression")
	}
	return funcKey{
		receiver: receiver,
		fnPC:     v.Pointer(),
	}
}

// Manager manages a collection of mock Invokers keyed by function identity.
//
// Manager is NOT goroutine-safe.
// All mock registrations must be completed before any concurrent logic starts.
type Manager struct {
	mockers map[funcKey][]Invoker
}

// NewManager creates and initializes a new Manager.
func NewManager() *Manager {
	m := &Manager{}
	m.Reset()
	return m
}

// Reset removes all registered mockers from the Manager.
func (r *Manager) Reset() {
	r.mockers = make(map[funcKey][]Invoker)
}

// addInvoker registers an Invoker for a specific function.
//
// receiver semantics:
//
//   - receiver == nil:
//     fn is a top-level function or a method expression
//     (e.g. Get, (*Client).Get)
//
//   - receiver != nil:
//     fn represents an instance method and is only used by
//     generated code for interface mocking.
//
// This method does not perform any deduplication; Invokers are
// evaluated in registration order.
func (r *Manager) addInvoker(receiver any, fn any, i Invoker) {
	k := newFuncKey(receiver, fn)
	r.mockers[k] = append(r.mockers[k], i)
}

// Invoke looks up and executes a mock Invoker for the given function call.
//
// Matching rules:
//   - The function is identified by its program counter (PC)
//   - receiver must match exactly (nil or the same instance)
//
// The Invokers are evaluated in registration order.
// The first Invoker whose Invoke method returns ok == true is selected.
// Its return values are returned immediately.
func Invoke(r *Manager, receiver any, fn any, params ...any) ([]any, bool) {
	k := newFuncKey(receiver, fn)
	for _, m := range r.mockers[k] {
		if ret, ok := m.Invoke(params); ok {
			return ret, true
		}
	}
	return nil, false
}

// InvokeContext retrieves the Manager from the context and invokes a mock.
//
// fn must be:
//   - a top-level function, or
//   - a method expression with receiver type (e.g. (*Client).Get)
//
// It must NOT be an instance method value (e.g. c.Get).
//
// Examples:
//
//	func Get(ctx context.Context, req *Request) (*Response, error)
//	→ fn is Get
//
//	func (c *Client) Get(ctx context.Context, req *Request) (*Response, error)
//	→ fn is (*Client).Get
//
// InvokeContext is not used for interface mocking.
// It only supports ordinary functions or methods with explicit receivers.
func InvokeContext(ctx context.Context, fn any, params ...any) ([]any, bool) {
	if r, ok := ctx.Value(&managerKey).(*Manager); ok {
		return Invoke(r, nil, fn, params...)
	}
	return nil, false
}

// Unbox1 extracts a single return value from a mock result slice.
//
// It panics if the number of return values is not exactly 1.
// Type assertion failures result in the zero value of the target type.
func Unbox1[R1 any](ret []any) (r1 R1) {
	if len(ret) == 1 {
		r1, _ = ret[0].(R1)
	} else {
		panic(fmt.Sprintf("expected 1 return value, but got %d", len(ret)))
	}
	return
}

// Unbox2 extracts two return values from a mock result slice.
//
// It panics if the number of return values is not exactly 2.
// Type assertion failures result in the zero value of the target type.
func Unbox2[R1, R2 any](ret []any) (r1 R1, r2 R2) {
	if len(ret) == 2 {
		r1, _ = ret[0].(R1)
		r2, _ = ret[1].(R2)
	} else {
		panic(fmt.Sprintf("expected 2 return values, but got %d", len(ret)))
	}
	return
}

// Unbox3 extracts three return values from a mock result slice.
//
// It panics if the number of return values is not exactly 3.
// Type assertion failures result in the zero value of the target type.
func Unbox3[R1, R2, R3 any](ret []any) (r1 R1, r2 R2, r3 R3) {
	if len(ret) == 3 {
		r1, _ = ret[0].(R1)
		r2, _ = ret[1].(R2)
		r3, _ = ret[2].(R3)
	} else {
		panic(fmt.Sprintf("expected 3 return values, but got %d", len(ret)))
	}
	return
}

// Unbox4 extracts four return values from a mock result slice.
//
// It panics if the number of return values is not exactly 4.
// Type assertion failures result in the zero value of the target type.
func Unbox4[R1, R2, R3, R4 any](ret []any) (r1 R1, r2 R2, r3 R3, r4 R4) {
	if len(ret) == 4 {
		r1, _ = ret[0].(R1)
		r2, _ = ret[1].(R2)
		r3, _ = ret[2].(R3)
		r4, _ = ret[3].(R4)
	} else {
		panic(fmt.Sprintf("expected 4 return values, but got %d", len(ret)))
	}
	return
}

// Unbox5 extracts five return values from a mock result slice.
//
// It panics if the number of return values is not exactly 5.
// Type assertion failures result in the zero value of the target type.
func Unbox5[R1, R2, R3, R4, R5 any](ret []any) (r1 R1, r2 R2, r3 R3, r4 R4, r5 R5) {
	if len(ret) == 5 {
		r1, _ = ret[0].(R1)
		r2, _ = ret[1].(R2)
		r3, _ = ret[2].(R3)
		r4, _ = ret[3].(R4)
		r5, _ = ret[4].(R5)
	} else {
		panic(fmt.Sprintf("expected 5 return values, but got %d", len(ret)))
	}
	return
}
