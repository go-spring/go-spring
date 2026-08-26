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
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/testing/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeAdmissionExec is a resilience.Executor stub that admits the first
// "allowed" calls and rejects everything beyond with ErrRateLimited — the
// admission-control behavior a rate-limit policy produces.
type fakeAdmissionExec struct {
	calls   atomic.Int32
	allowed int32
}

func (f *fakeAdmissionExec) Execute(ctx context.Context, _ string, fn func(context.Context) error) error {
	if f.calls.Add(1) > f.allowed {
		return resilience.ErrRateLimited
	}
	return fn(ctx)
}
func (f *fakeAdmissionExec) Close() error                    { return nil }
func (f *fakeAdmissionExec) Refresh(resilience.Policy) error { return nil }

// admissionRegistry dispatches the process-wide executor provider seam by
// resource label. resilience.RegisterExecutorProvider cannot be un-registered
// and memoizes executors per label, so each test installs its fake under a
// UNIQUE label (a unique Addr) and unknown labels resolve to the transparent
// no-op executor — mirroring "governance not configured".
var (
	admissionOnce    sync.Once
	admissionFakes   sync.Map // resource label -> resilience.Executor
	uniqueAddrSerial atomic.Int64
)

func uniqueTestAddr(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, uniqueAddrSerial.Add(1))
}

// TestResilienceUnaryInterceptor_Unit drives the interceptor directly: the
// admitted call reaches the handler and returns its response; the rejected call
// surfaces the executor's error and the handler never runs.
func TestResilienceUnaryInterceptor_Unit(t *testing.T) {
	exec := &fakeAdmissionExec{allowed: 1}
	ic := resilienceUnaryInterceptor(exec, "grpc:test")
	info := &grpc.UnaryServerInfo{FullMethod: "/demo.Service/Echo"}

	hits := 0
	resp, err := ic(context.Background(), nil, info,
		func(context.Context, any) (any, error) { hits++; return "ok", nil })
	assert.That(t, err).Nil()
	assert.That(t, resp).Equal("ok")
	assert.That(t, hits).Equal(1)

	_, err = ic(context.Background(), nil, info,
		func(context.Context, any) (any, error) { hits++; return "ok", nil })
	// The sentinel is mapped to its semantic status (ResourceExhausted); the
	// message keeps the original text so log readers still see "rate limited".
	assert.That(t, status.Code(err)).Equal(codes.ResourceExhausted)
	assert.String(t, err.Error()).Contains("rate limited")
	assert.That(t, hits).Equal(1) // rejected before the handler
}

// TestAdmission_EndToEndOverBufconn exercises the full assembly path a real
// request takes: buildOptions wires the resilience interceptor resolved from
// the neutral resilience.ExecutorFor seam, so a policy that admits only the
// first call lets exactly one RPC through to the health handler and rejects
// the rest.
//
// Admission rejections are mapped to semantic status codes (see
// mapAdmissionError): a rate-limit rejection crosses the wire as
// ResourceExhausted with message "resilience: rate limited", so consumers
// branching on status code can recognise throttling.
func TestAdmission_EndToEndOverBufconn(t *testing.T) {
	addr := uniqueTestAddr("admission-e2e")
	resource := resilience.ResourceLabel("grpc", addr)
	exec := &fakeAdmissionExec{allowed: 1}
	admissionFakes.Store(resource, exec)

	admissionOnce.Do(func() {
		resilience.RegisterExecutorProvider(func(label string) resilience.Executor {
			if v, ok := admissionFakes.Load(label); ok {
				return v.(resilience.Executor)
			}
			return nil // unknown label => transparent no-op executor
		})
	})

	s := NewSimpleGrpcServer(Config{Addr: addr}, func(*grpc.Server) {})
	opts, err := s.buildOptions()
	assert.That(t, err).Nil()

	srv := grpc.NewServer(opts...)
	hs := health.NewServer()
	healthpb.RegisterHealthServer(srv, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	lis := bufconn.Listen(16 * 1024)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.That(t, err).Nil()
	defer conn.Close()

	cl := healthpb.NewHealthClient(conn)
	resp, err := cl.Check(ctx, &healthpb.HealthCheckRequest{})
	assert.That(t, err).Nil()
	assert.That(t, resp.GetStatus()).Equal(healthpb.HealthCheckResponse_SERVING)

	// Beyond the allowance: rejected before the handler, with the semantic
	// ResourceExhausted code (the sentinel itself is not transported, so
	// match on the message).
	_, err = cl.Check(ctx, &healthpb.HealthCheckRequest{})
	assert.That(t, err != nil).True()
	assert.String(t, status.Convert(err).Message()).Contains("rate limited")
	assert.That(t, exec.calls.Load()).Equal(int32(2))
	assert.That(t, status.Code(err)).Equal(codes.ResourceExhausted)
}

// TestAdmission_NoProviderForLabelIsTransparent pins the governance-off
// default: a label the provider does not know resolves to the no-op executor,
// so every RPC runs the handler untouched.
func TestAdmission_NoProviderForLabelIsTransparent(t *testing.T) {
	addr := uniqueTestAddr("admission-unarmed")
	s := NewSimpleGrpcServer(Config{Addr: addr}, func(*grpc.Server) {})
	ri, ok := s.buildResilienceInterceptors()
	assert.That(t, ok).True()

	hits := 0
	for i := 0; i < 3; i++ {
		resp, err := ri.unary(context.Background(), nil,
			&grpc.UnaryServerInfo{FullMethod: "/demo.Service/Echo"},
			func(context.Context, any) (any, error) { hits++; return "ok", nil })
		assert.That(t, err).Nil()
		assert.That(t, resp).Equal("ok")
	}
	assert.That(t, hits).Equal(3)
}
