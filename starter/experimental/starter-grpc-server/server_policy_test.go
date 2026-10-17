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

package StarterGrpcServer

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/runtime"

	"go-spring.org/cloud/runtime/controlplane/governance"
	"go-spring.org/cloud/runtime/dataplane/fault"
	"go-spring.org/cloud/runtime/dataplane/guard"
	"go-spring.org/cloud/runtime/dataplane/loadbalance"
	"go-spring.org/cloud/runtime/dataplane/resilience"
	"go-spring.org/stdlib/testing/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// testCenter builds the runtime the server takes. The resilience
// authorities are real unarmed managers (construction dispatches into every
// authority), and the injector is a real inert one, mirroring the DI wiring
// where every bean is always present (an absent bean fails startup).
func testCenter() *runtime.Runtime {
	driver := resilience.NewDefaultDriver(nil)
	resClient := resilience.NewClientManager(driver)
	ctr, err := runtime.New(resClient, loadbalance.NewManager(),
		fault.NewClientManager(nil), fault.NewServerManager(nil), nil, governance.NewPushSource(governance.Config{}))
	if err != nil {
		panic(err)
	}
	return ctr
}

// armRateLimit pushes a one-request rate budget into the center's guard authority.
func armRateLimit(t *testing.T, ctr *runtime.Runtime) {
	t.Helper()
	assert.That(t, ctr.GuardServer().Apply(guard.Settings{
		Default: guard.Policy{RateLimit: 1, Burst: 1},
	})).Nil()
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
	ctr := testCenter()
	armRateLimit(t, ctr)
	ic := resilienceUnaryInterceptor(ctr.InboundFor("grpc", "grpc:test"), "grpc:test")
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

	ctr := testCenter()
	s := NewSimpleGrpcServer(Config{Addr: addr}, func(*grpc.Server) {}, nil, nil, ctr, nil)
	// Arm AFTER constructing the server: the server captures its Inbound at
	// wiring time, and the guard follows the manager's later pushes.
	armRateLimit(t, ctr)
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
	assert.That(t, status.Code(err)).Equal(codes.ResourceExhausted)
}

// TestServerPolicy_UnarmedManagerIsTransparent pins the governance-off default: no
// manager bean (nil) is normalized to an unarmed manager, whose executor is a
// no-op, so every RPC runs the handler untouched.
func TestServerPolicy_UnarmedManagerIsTransparent(t *testing.T) {
	addr := uniqueTestAddr("inbound-unarmed")
	s := NewSimpleGrpcServer(Config{Addr: addr}, func(*grpc.Server) {}, nil, nil, testCenter(), nil)
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
