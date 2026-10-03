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
	"go-spring.org/cloud/chain"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeServerExec is a chain.Executor stub that admits the first
// "allowed" calls and rejects everything beyond with ErrRateLimited — the
// inbound-control behavior a rate-limit inbound model produces.
type fakeServerExec struct {
	calls   atomic.Int32
	allowed int32
}

func (f *fakeServerExec) Execute(ctx context.Context, fn func(context.Context) error) error {
	if f.calls.Add(1) > f.allowed {
		return chain.ErrRateLimited
	}
	return fn(ctx)
}
func (f *fakeServerExec) Close() error { return nil }

// fakeServerDriver is a resilience.Driver that hands out the fake executor
// for every inbound model, so a Manager armed with it resolves each label to
// the same inbound behavior — the test-local stand-in for a governance
// document's server block.
type fakeServerDriver struct{ exec chain.Executor }

func (d fakeServerDriver) NewClientExecutor(service string, p resilience.ClientPolicy) (chain.Executor, error) {
	return resilience.NewDefaultDriver(nil).NewClientExecutor(service, p)
}

func (d fakeServerDriver) NewServerExecutor(service string, a resilience.ServerPolicy) (chain.Executor, error) {
	return d.exec, nil
}

// armedManager returns a Manager armed with the fake executor: the inbound
// resolver hands every label the zero model, which the fake driver turns into the
// fake executor, so a label the manager resolves runs through it. Without a
// resolver the manager stays pass-through (see Manager.build).
func armedManager(t *testing.T, exec chain.Executor) *resilience.Manager {
	t.Helper()
	mgr := resilience.NewManager(map[string]resilience.Driver{"fake": fakeServerDriver{exec: exec}})
	assert.That(t, mgr.Apply(resilience.Settings{
		Enabled:             true,
		Driver:              "fake",
		ResolveServerPolicy: func(string) resilience.ServerPolicy { return resilience.ServerPolicy{} },
	})).Nil()
	return mgr
}

// testCenter bundles an authority into the governance center the server takes.
func testCenter(mgr *resilience.Manager) *governance.Center {
	return governance.NewCenter(governance.Config{}, mgr, nil, nil, nil, nil)
}

func uniqueTestAddr(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, addrSerial.Add(1))
}

// addrSerial keeps each test's server address distinct so two servers never
// bind the same endpoint within one run.
var addrSerial atomic.Int64

// TestResilienceUnaryInterceptor_Unit drives the interceptor directly: the
// admitted call reaches the handler and returns its response; the rejected call
// surfaces the executor's error and the handler never runs.
func TestResilienceUnaryInterceptor_Unit(t *testing.T) {
	exec := &fakeServerExec{allowed: 1}
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

// TestServerPolicy_EndToEndOverBufconn exercises the full assembly path a real
// request takes: buildOptions wires the resilience interceptor built from the
// injected Manager, so a policy that admits only the first call lets exactly one
// RPC through to the health handler and rejects the rest.
//
// ServerPolicy rejections are mapped to semantic status codes (see
// mapServerPolicyError): a rate-limit rejection crosses the wire as
// ResourceExhausted with message "resilience: rate limited", so consumers
// branching on status code can recognise throttling.
func TestServerPolicy_EndToEndOverBufconn(t *testing.T) {
	addr := uniqueTestAddr("inbound-e2e")
	exec := &fakeServerExec{allowed: 1}
	mgr := armedManager(t, exec)

	s := NewSimpleGrpcServer(Config{Addr: addr}, func(*grpc.Server) {}, nil, nil, testCenter(mgr), nil)
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

// TestServerPolicy_UnarmedManagerIsTransparent pins the governance-off default: no
// manager bean (nil) is normalized to an unarmed manager, whose executor is a
// no-op, so every RPC runs the handler untouched.
func TestServerPolicy_UnarmedManagerIsTransparent(t *testing.T) {
	addr := uniqueTestAddr("inbound-unarmed")
	s := NewSimpleGrpcServer(Config{Addr: addr}, func(*grpc.Server) {}, nil, nil, testCenter(nil), nil)
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
