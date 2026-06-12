/*
 * Copyright 2024 The Go-Spring Authors.
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

// Package goutil provides utilities for running goroutines safely with
// built-in panic recovery.
//
// In Go, a panic that occurs inside a goroutine will terminate the entire
// process if it is not recovered. This package wraps goroutine execution
// with a deferred recover handler to prevent such crashes.
//
// When a panic is recovered, a global OnPanic callback is invoked, allowing
// applications to log the panic, emit metrics, or trigger alerts. This makes
// failures in concurrent code easier to observe and diagnose.
package goutil

import (
	"context"
	"fmt"
	"runtime/debug"

	"go-spring.org/stdlib/errutil"
)

// PanicInfo contains information captured when a panic is recovered.
type PanicInfo struct {
	Panic any
	Stack []byte
}

// OnPanic is a global callback invoked whenever a panic is recovered inside
// a goroutine launched by this package.
//
// By default, it prints the panic value and stack trace to stdout.
// Applications may override this function during initialization to provide
// custom logging, metrics, or alerting behavior.
var OnPanic = func(ctx context.Context, info PanicInfo) {
	fmt.Printf("[PANIC] %v\n%s\n", info.Panic, info.Stack)
}

// CancelMode controls how the context passed to a goroutine handles
// cancellation relative to its parent context.
type CancelMode int

const (
	// InheritCancel means the goroutine receives the original context
	// and therefore inherits its cancellation and deadline.
	InheritCancel CancelMode = iota

	// DetachCancel means the goroutine receives a context created with
	// context.WithoutCancel, so cancellation of the parent context does
	// not propagate to the goroutine.
	DetachCancel
)

// Status represents the lifecycle of a goroutine launched by the Go function.
// It provides a synchronization point to wait for the goroutine to finish.
type Status struct {
	ch chan struct{}
}

// newStatus creates a new Status instance.
func newStatus() *Status {
	return &Status{ch: make(chan struct{})}
}

// done signals that the goroutine has completed.
func (s *Status) done() {
	close(s.ch)
}

// Wait blocks until the associated goroutine completes.
func (s *Status) Wait() {
	<-s.ch
}

// Go launches a new goroutine to execute f with panic recovery enabled.
//
// Any panic raised during execution of f is recovered, and the global
// OnPanic handler is invoked if it is not nil.
//
// The provided context is passed to both f and OnPanic. Context cancellation
// is cooperative: the goroutine will NOT stop automatically when ctx is
// canceled. The function f must observe ctx.Done() and return explicitly.
//
// If mode is DetachCancel, f receives a context derived using
// context.WithoutCancel. In that case, cancellation and deadlines of the
// parent context will not propagate to the goroutine.
func Go(ctx context.Context, f func(ctx context.Context), mode CancelMode) *Status {
	if mode == DetachCancel && ctx != nil {
		ctx = context.WithoutCancel(ctx)
	}
	s := newStatus()
	go func() {
		defer s.done()
		defer func() {
			if r := recover(); r != nil {
				if OnPanic != nil {
					OnPanic(ctx, PanicInfo{r, debug.Stack()})
				}
			}
		}()
		f(ctx)
	}()
	return s
}

// ValueStatus represents the execution status of a goroutine that returns
// a value and an error.
type ValueStatus[T any] struct {
	ch  chan struct{}
	val T
	err error
}

// newValueStatus creates a new ValueStatus instance.
func newValueStatus[T any]() *ValueStatus[T] {
	return &ValueStatus[T]{ch: make(chan struct{})}
}

// done signals that the goroutine has completed.
func (s *ValueStatus[T]) done() {
	close(s.ch)
}

// Wait blocks until the goroutine completes and returns the produced
// value and error.
//
// If a panic occurred during execution, the returned error will describe
// the recovered panic.
func (s *ValueStatus[T]) Wait() (T, error) {
	<-s.ch
	return s.val, s.err
}

type GoValueFunc[T any] func(ctx context.Context) (T, error)

// GoValue launches a new goroutine to execute f, capturing its returned
// value and error, with panic recovery enabled.
//
// If f panics, the panic is recovered, reported via OnPanic, and converted
// into an error that is returned by Wait. In this case, the returned value
// is the zero value of T.
//
// As with Go, context cancellation is cooperative: f must observe ctx.Done()
// if early termination is required.
//
// If mode is DetachCancel, f receives a context derived using
// context.WithoutCancel. In that case, cancellation and deadlines of the
// parent context will not propagate to the goroutine.
func GoValue[T any](ctx context.Context, f GoValueFunc[T], mode CancelMode) *ValueStatus[T] {
	if mode == DetachCancel && ctx != nil {
		ctx = context.WithoutCancel(ctx)
	}
	s := newValueStatus[T]()
	go func() {
		defer s.done()
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				if OnPanic != nil {
					OnPanic(ctx, PanicInfo{r, stack})
				}
				s.err = errutil.Explain(nil, "panic recovered: %v\n%s", r, stack)
			}
		}()
		s.val, s.err = f(ctx)
	}()
	return s
}
