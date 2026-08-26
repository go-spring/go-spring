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

package StarterGrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/stdlib/testing/assert"
	"google.golang.org/grpc"
)

// TestFaultUnaryInterceptor_NoInjectorPassthrough pins the zero-config
// transparency claim: with no injector registered the always-installed
// interceptor is a pass-through.
func TestFaultUnaryInterceptor_NoInjectorPassthrough(t *testing.T) {
	ic := FaultUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/demo.Service/Echo"}
	resp, err := ic(context.Background(), nil, info,
		func(context.Context, any) (any, error) { return "ok", nil })
	assert.That(t, err).Nil()
	assert.That(t, resp).Equal("ok")
}

// TestFaultUnaryInterceptor_InjectsErrorAtRate1 covers the "set fire" path:
// with a registered injector at Rate 1 every call returns the injected error
// and the handler never runs; hot-toggling the injector off (SetConfig on the
// live injector, no restart) restores pass-through on the next call because
// the interceptor resolves fault.InjectorFor() per call.
func TestFaultUnaryInterceptor_InjectsErrorAtRate1(t *testing.T) {
	in := fault.NewInjector(fault.Config{Enabled: true, Rate: 1, Error: "generic"})
	fault.RegisterInjector(in)
	t.Cleanup(func() { fault.RegisterInjector(nil) })

	ic := FaultUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/demo.Service/Echo"}
	hits := 0

	_, err := ic(context.Background(), nil, info,
		func(context.Context, any) (any, error) { hits++; return "ok", nil })
	assert.That(t, errors.Is(err, fault.ErrInjected)).True()
	assert.That(t, hits).Equal(0)

	// Hot-toggle off: the same interceptor passes the next call through.
	in.SetConfig(fault.Config{})
	resp, err := ic(context.Background(), nil, info,
		func(context.Context, any) (any, error) { hits++; return "ok", nil })
	assert.That(t, err).Nil()
	assert.That(t, resp).Equal("ok")
	assert.That(t, hits).Equal(1)
}

// TestFaultUnaryInterceptor_InjectsLatency verifies the latency knob: the
// injected sleep happens before the handler, and the call still succeeds.
func TestFaultUnaryInterceptor_InjectsLatency(t *testing.T) {
	in := fault.NewInjector(fault.Config{Enabled: true, Latency: 50 * time.Millisecond})
	fault.RegisterInjector(in)
	t.Cleanup(func() { fault.RegisterInjector(nil) })

	ic := FaultUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/demo.Service/Echo"}
	start := time.Now()
	resp, err := ic(context.Background(), nil, info,
		func(context.Context, any) (any, error) { return "ok", nil })
	assert.That(t, err).Nil()
	assert.That(t, resp).Equal("ok")
	assert.That(t, time.Since(start) >= 50*time.Millisecond).True()
}

// TestFaultStreamInterceptor_InjectThenDisable is the streaming twin: an
// injected error aborts the stream handler, and disabling the injector
// restores the pass-through.
func TestFaultStreamInterceptor_InjectThenDisable(t *testing.T) {
	in := fault.NewInjector(fault.Config{Enabled: true, Rate: 1, Error: "generic"})
	fault.RegisterInjector(in)
	t.Cleanup(func() { fault.RegisterInjector(nil) })

	ic := FaultStreamInterceptor()
	info := &grpc.StreamServerInfo{FullMethod: "/demo.Service/Chat"}
	hits := 0

	err := ic(nil, stubServerStream{}, info,
		func(any, grpc.ServerStream) error { hits++; return nil })
	assert.That(t, errors.Is(err, fault.ErrInjected)).True()
	assert.That(t, hits).Equal(0)

	in.SetConfig(fault.Config{})
	err = ic(nil, stubServerStream{}, info,
		func(any, grpc.ServerStream) error { hits++; return nil })
	assert.That(t, err).Nil()
	assert.That(t, hits).Equal(1)
}
