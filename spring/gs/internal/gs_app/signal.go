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

package gs_app

import (
	"sync"
	"sync/atomic"
)

// ReadySignalImpl is a synchronization helper used to indicate
// when an application is ready to serve requests.
type ReadySignalImpl struct {
	wg      sync.WaitGroup
	mu      sync.Mutex
	ch      chan struct{}
	b       atomic.Bool
	pending int
}

// NewReadySignal creates and returns a new ReadySignalImpl instance.
func NewReadySignal() *ReadySignalImpl {
	return &ReadySignalImpl{
		ch: make(chan struct{}),
	}
}

// Add increments the WaitGroup counter.
func (s *ReadySignalImpl) Add() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending++
	s.wg.Add(1)
}

// TriggerAndWait marks an operation as done by decrementing the WaitGroup
// counter, and then returns the readiness signal channel for waiting.
func (s *ReadySignalImpl) TriggerAndWait() <-chan struct{} {
	s.done()
	return s.ch
}

// Intercepted returns true if the signal has been intercepted.
func (s *ReadySignalImpl) Intercepted() bool {
	return s.b.Load()
}

// Intercept marks the signal as intercepted.
func (s *ReadySignalImpl) Intercept() {
	s.b.Store(true)
	s.done()
}

// Wait blocks until all WaitGroup counters reach zero.
func (s *ReadySignalImpl) Wait() {
	s.wg.Wait()
}

// Close closes the signal channel, notifying all goroutines waiting on it.
func (s *ReadySignalImpl) Close() {
	close(s.ch)
}

func (s *ReadySignalImpl) done() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == 0 {
		return
	}
	s.pending--
	s.wg.Done()
}
