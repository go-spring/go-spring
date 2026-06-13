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

// ReadySignalImpl coordinates readiness for all servers.
// Each server gets a single-use ServerReadySignal via Add.
type ReadySignalImpl struct {
	wg sync.WaitGroup
	ch chan struct{}
	b  atomic.Bool
}

// NewReadySignal creates and returns a new ReadySignalImpl instance.
func NewReadySignal() *ReadySignalImpl {
	return &ReadySignalImpl{
		ch: make(chan struct{}),
	}
}

// Add registers one server and returns its single-use readiness signal handle.
func (c *ReadySignalImpl) Add() *ServerReadySignal {
	c.wg.Add(1)
	return &ServerReadySignal{c: c}
}

// Intercepted returns true if any server signal has been intercepted.
func (c *ReadySignalImpl) Intercepted() bool {
	return c.b.Load()
}

// Wait blocks until all server signals are triggered or intercepted.
func (c *ReadySignalImpl) Wait() {
	c.wg.Wait()
}

// Close closes the signal channel, notifying all goroutines waiting on it.
func (c *ReadySignalImpl) Close() {
	close(c.ch)
}

// ServerReadySignal is a per-server readiness signal handle.
type ServerReadySignal struct {
	c *ReadySignalImpl
	o sync.Once
}

// TriggerAndWait marks the server as ready, then returns the shared readiness
// channel for waiting until all servers have reported readiness.
func (s *ServerReadySignal) TriggerAndWait() <-chan struct{} {
	s.o.Do(s.c.wg.Done)
	return s.c.ch
}

// Intercept marks the signal as intercepted and releases this server's ready wait.
func (s *ServerReadySignal) Intercept() {
	s.c.b.Store(true)
	s.o.Do(s.c.wg.Done)
}
