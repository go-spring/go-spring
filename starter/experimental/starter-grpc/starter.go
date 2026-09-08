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
	"net"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/tlsconf"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

func init() {
	gs.Provide(
		NewSimpleGrpcServer,
		gs.IndexArg(0, gs.TagArg("${spring.grpc.server}")),
	).Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.grpc.server.addr"))

	// Capture the container's named discovery backend beans (bean name = label)
	// into the directory the gsdiscovery resolver resolves "gsdiscovery://<label>
	// /<service>" targets against. Collected at assembly time so target
	// resolution never consults process-global state at runtime. Exported as a
	// Rooter so the map is captured even when nothing autowires the hook.
	gs.Provide(newDiscoveryBackendsHook,
		gs.IndexArg(0, gs.TagArg("?")),
	).Export(gs.As[gs.Rooter]()).Caller(1)
}

// newDiscoveryBackendsHook installs every named discovery.Discovery bean into
// the gsdiscovery resolver's label directory (see balancer.go). The map is nil
// when the app declares no backend beans — dials then fail loudly on the label.
type discoveryBackendsHook struct{}

func newDiscoveryBackendsHook(backends map[string]discovery.Discovery) (*discoveryBackendsHook, error) {
	SetDiscoveryBackends(backends)
	return &discoveryBackendsHook{}, nil
}

// ServiceRegister registers services on a grpc.Server.
type ServiceRegister func(svr *grpc.Server)

// KeepaliveConfig tunes server-side keepalive enforcement. Zero values leave
// the corresponding gRPC default in place.
type KeepaliveConfig struct {
	Time              time.Duration `value:"${time:=0}"`
	Timeout           time.Duration `value:"${timeout:=0}"`
	MaxConnectionIdle time.Duration `value:"${maxConnectionIdle:=0}"`
	MaxConnectionAge  time.Duration `value:"${maxConnectionAge:=0}"`
}

// HealthConfig toggles the standard grpc_health_v1 health service. It is
// enabled by default because it is the conventional way to expose gRPC
// readiness to load balancers and probes.
type HealthConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// LoadTestConfig toggles inbound load-test traffic identification on the gRPC
// server. When enabled (the default — it costs one metadata lookup per RPC) the
// LoadTest interceptors read the marker key (x-loadtest) off the incoming
// metadata and tag the handler context, so tracing, metrics, resilience and the
// handler itself can branch on traffic.IsLoadTest(ctx). It is the gRPC inbound
// counterpart to cloud/governance/traffic's outbound carrier injection.
type LoadTestConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// Config defines gRPC server configuration.
type Config struct {
	Addr                 string                `value:"${addr}"`
	ConnectionTimeout    time.Duration         `value:"${connectionTimeout:=0}"`
	MaxRecvMsgSize       int                   `value:"${maxRecvMsgSize:=0}"`
	MaxSendMsgSize       int                   `value:"${maxSendMsgSize:=0}"`
	MaxConcurrentStreams uint32                `value:"${maxConcurrentStreams:=0}"`
	Keepalive            KeepaliveConfig       `value:"${keepalive}"`
	TLS                  tlsconf.TLSConfig     `value:"${tls}"`
	Health               HealthConfig          `value:"${health}"`
	LoadTest             LoadTestConfig        `value:"${loadtest}"`
	Observer             ObserverConfig        `value:"${observer}"`
}

// ObserverConfig groups the built-in observability interceptors the starter can
// install on the grpc.Server. Tracing and Metrics ride the OTel globals that
// starter-otel installs; they are on by default because the interceptors are
// no-ops when starter-otel is not imported — enabling them upfront saves a
// config change when adopting OTel.
type ObserverConfig struct {
	Tracing TracingConfig `value:"${tracing}"`
	Metrics MetricsConfig `value:"${metrics}"`
}

// TracingConfig toggles the gRPC tracing interceptors. On by default.
type TracingConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// MetricsConfig toggles the gRPC metrics interceptors. On by default.
type MetricsConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// SimpleGrpcServer adapts a grpc.Server to the Go-Spring server lifecycle.
type SimpleGrpcServer struct {
	cfg Config
	reg ServiceRegister
	svr *grpc.Server
}

// NewSimpleGrpcServer creates a SimpleGrpcServer from ${spring.grpc.server}
// configuration. Inbound admission protection (rate-limit / breaker) is
// resolved inside buildResilienceInterceptors via the neutral
// resilience.ExecutorFor seam, so this server has no coupling to cloud/governance.
func NewSimpleGrpcServer(cfg Config, reg ServiceRegister) *SimpleGrpcServer {
	log.Debugf(context.Background(), log.TagAppDef, "grpc server created addr=%s", cfg.Addr)
	return &SimpleGrpcServer{cfg: cfg, reg: reg}
}

// buildOptions translates the bound Config into grpc.ServerOption values.
func (s *SimpleGrpcServer) buildOptions() ([]grpc.ServerOption, error) {
	var opts []grpc.ServerOption
	if s.cfg.ConnectionTimeout > 0 {
		opts = append(opts, grpc.ConnectionTimeout(s.cfg.ConnectionTimeout))
	}
	if s.cfg.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(s.cfg.MaxRecvMsgSize))
	}
	if s.cfg.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(s.cfg.MaxSendMsgSize))
	}
	if s.cfg.MaxConcurrentStreams > 0 {
		opts = append(opts, grpc.MaxConcurrentStreams(s.cfg.MaxConcurrentStreams))
	}

	ka := s.cfg.Keepalive
	if ka.Time > 0 || ka.Timeout > 0 || ka.MaxConnectionIdle > 0 || ka.MaxConnectionAge > 0 {
		opts = append(opts, grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:              ka.Time,
			Timeout:           ka.Timeout,
			MaxConnectionIdle: ka.MaxConnectionIdle,
			MaxConnectionAge:  ka.MaxConnectionAge,
		}))
	}

	if s.cfg.TLS.Enabled {
		tlsCfg, err := s.cfg.TLS.BuildServer()
		if err != nil {
			return nil, errutil.Explain(err, "grpc: build TLS")
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	// Observability + resilience interceptors are installed via chained
	// ServerOptions so they wrap every registered service and compose with each
	// other. NOTE: grpc.UnaryInterceptor is a setter (the last call wins), so the
	// previous per-interceptor grpc.UnaryInterceptor calls silently shadowed each
	// other — ChainUnaryInterceptor is used here to fix that and to admit the
	// resilience interceptor. Tracing/metrics ride the OTel globals, no-op
	// without starter-otel.
	var unary []grpc.UnaryServerInterceptor
	var stream []grpc.StreamServerInterceptor
	// User-registered interceptors are outermost (first in the chain), mirroring
	// starter-gin's EngineMiddleware: an app guard sees the request before the
	// built-in stack and can short-circuit before any work is observed. Built-ins
	// follow: LoadTest, Tracing, Metrics, Resilience, then the handler.
	unary = append(unary, currentUserUnary()...)
	stream = append(stream, currentUserStream()...)
	// LoadTest identification is outermost of the built-ins so the marker is on
	// the context before tracing, metrics, resilience or the handler run, letting
	// every downstream layer branch on traffic.IsLoadTest(ctx). A no-op when the
	// inbound metadata lacks the marker key.
	if s.cfg.LoadTest.Enabled {
		unary = append(unary, LoadTestUnaryInterceptor())
		stream = append(stream, LoadTestStreamInterceptor())
	}
	if s.cfg.Observer.Tracing.Enabled {
		unary = append(unary, TracingUnaryInterceptor())
		stream = append(stream, TracingStreamInterceptor())
	}
	if s.cfg.Observer.Metrics.Enabled {
		unary = append(unary, MetricsUnaryInterceptor())
		stream = append(stream, MetricsStreamInterceptor())
	}
	if ropts, ok := s.buildResilienceInterceptors(); ok {
		unary = append(unary, ropts.unary)
	}
	// Fault injection (always installed), innermost so an injected error flows
	// back through tracing/metrics/resilience and is observed. The injector is
	// resolved from the neutral [fault.InjectorFor] seam on each call (nil-safe:
	// a transparent pass-through when fault is off), so fault can be hot-toggled
	// at runtime without a restart.
	unary = append(unary, FaultUnaryInterceptor())
	stream = append(stream, FaultStreamInterceptor())
	// Recovery (always installed), innermost so a converted panic flows back
	// through tracing/metrics/resilience as a codes.Internal error and is
	// observed — grpc-go recovers handler panics nowhere by itself.
	unary = append(unary, RecoverUnaryInterceptor())
	stream = append(stream, RecoverStreamInterceptor())
	if len(unary) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(unary...))
	}
	if len(stream) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(stream...))
	}
	return opts, nil
}

// Run starts the gRPC server after Go-Spring signals readiness.
func (s *SimpleGrpcServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	opts, err := s.buildOptions()
	if err != nil {
		return err
	}
	s.svr = grpc.NewServer(opts...)

	// Mount the standard health service so probes and load balancers can query
	// serving status via grpc_health_v1.
	if s.cfg.Health.Enabled {
		hs := health.NewServer()
		healthpb.RegisterHealthServer(s.svr, hs)
		hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	}

	s.reg(s.svr)

	listener, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "grpc server failed to listen on %s: %v", s.cfg.Addr, err)
		return errutil.Explain(err, "failed to listen on %s", s.cfg.Addr)
	}
	<-sig.TriggerAndWait()
	log.Infof(ctx, log.TagAppDef, "grpc server starting on %s", s.cfg.Addr)
	if err = s.svr.Serve(listener); err != nil {
		log.Errorf(ctx, log.TagAppDef, "grpc server failed on %s: %v", s.cfg.Addr, err)
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	}
	return nil
}

// Stop gracefully stops the underlying gRPC server. grpc's GracefulStop
// takes no context, so ctx only tags the shutdown log.
func (s *SimpleGrpcServer) Stop(ctx context.Context) error {
	log.Infof(ctx, log.TagAppDef, "grpc server shutting down on %s", s.cfg.Addr)
	s.svr.GracefulStop()
	return nil
}
