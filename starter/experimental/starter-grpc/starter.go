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

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/security"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"
)

func init() {
	gs.Provide(
		NewSimpleGrpcServer,
		gs.IndexArg(0, gs.TagArg("${spring.grpc.server}")),
		gs.IndexArg(2, gs.TagArg("?")), // nullable []grpc.UnaryServerInterceptor beans
		gs.IndexArg(3, gs.TagArg("?")), // nullable []grpc.StreamServerInterceptor beans
		// The governance center is the family's sole injection point: it hands
		// out the resilience/fault/loadbalance authorities.
		gs.IndexArg(5, gs.TagArg("?")), // nullable traffic.Propagator bean
	).Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.grpc.server.addr"))

	// Capture the discovery directory (bean name = label) into the directory the
	// gsdiscovery resolver resolves "gsdiscovery://<label>/<service>" targets
	// against. The directory rides the governance center, so this hook injects
	// the center — the family's sole injection point — and reads the directory
	// off it. Collected at assembly time so target resolution never consults
	// process-global state at runtime. Exported as a Rooter so the directory is
	// captured even when nothing autowires the hook.
	gs.Provide(newDiscoveryBackendsHook).
		Export(gs.As[gs.Rooter]()).Caller(1)
}

// The client-side half: the gsdiscovery resolver and the built-in governance
// balancers (see balancer.go).
func init() {
	resolver.Register(discoveryResolverBuilder{})

	// One tracker per built-in strategy, all starting disabled.
	for _, s := range builtinStrategies {
		t := loadbalance.NewTracker(loadbalance.TrackerConfig{})
		registerBalancer(BalancerName(s), s, t, true)
		builtinTrackers = append(builtinTrackers, t)
	}

	// The install-and-hold bean that wires governance onto the built-in
	// balancers. gRPC's balancer registry builds balancers through a no-arg
	// constructor (see [newSelectionHook]), so the loadbalance manager cannot arrive
	// as a balancer constructor parameter: a bean receives it and subscribes on
	// their behalf, writing the resulting policy into the package handles
	// ([builtinTrackers], [governedBal]) the pickers read. Exported as a gs.Rooter
	// so gs instantiates it even though nothing injects it — without a
	// collected-type export an unreachable bean is never created and the policy
	// would never be applied.
	gs.Provide(newSelectionHook).
		Export(gs.As[gs.Rooter]()).Caller(1)
}

// newDiscoveryBackendsHook installs the center's discovery directory into the
// gsdiscovery resolver's label directory (see balancer.go). center is nil when
// no center is linked, which reads a nil directory — dials then fail loudly on
// the label.
type discoveryBackendsHook struct{}

func newDiscoveryBackendsHook(center *governance.Center) (*discoveryBackendsHook, error) {
	SetDiscoveryBackends(center.Discovery())
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
// LoadTest interceptors read the propagator's marker key off the incoming
// metadata and tag the handler context, so tracing, metrics, resilience and the
// handler itself can branch on the propagator's IsLoadTest(ctx). It is the gRPC
// inbound counterpart to cloud/traffic's outbound carrier injection.
type LoadTestConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// Config defines gRPC server configuration.
type Config struct {
	Addr                 string             `value:"${addr}"`
	ConnectionTimeout    time.Duration      `value:"${connectionTimeout:=0}"`
	MaxRecvMsgSize       int                `value:"${maxRecvMsgSize:=0}"`
	MaxSendMsgSize       int                `value:"${maxSendMsgSize:=0}"`
	MaxConcurrentStreams uint32             `value:"${maxConcurrentStreams:=0}"`
	Keepalive            KeepaliveConfig    `value:"${keepalive}"`
	TLS                  security.TLSConfig `value:"${tls}"`
	Health               HealthConfig       `value:"${health}"`
	LoadTest             LoadTestConfig     `value:"${loadtest}"`
	Observer             ObserverConfig     `value:"${observer}"`
}

// ObserverConfig groups the built-in observability interceptors the starter can
// install on the grpc.Server. Tracing and Metrics ride the OTel globals that
// starter-otel installs; they are on by default because the interceptors are
// no-ops when starter-otel is not imported — enabling them upfront saves a
// config change when adopting OTel.
//
// The per-call access log is NOT covered by these switches: it is installed
// always, because the RPC family requires every member to have one, and a
// toggle that can silently remove a required signal is a trap. Turning both
// switches off leaves the server logging one line per call, and nothing else.
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
	// User-contributed interceptors, injected as bean collections ([]grpc.UnaryServerInterceptor /
	// []grpc.StreamServerInterceptor) — per-container, no package-level stack.
	userUnary  []grpc.UnaryServerInterceptor
	userStream []grpc.StreamServerInterceptor
	// center is the governance center bean (nil when governance is not
	// imported); its fault authority backs the always-installed fault
	// interceptors and its resilience authority supplies the inbound policy.
	center *governance.Center
	// prop is the application's load-test convention bean (nil-normalized to
	// go-spring's default); the LoadTest interceptors read the inbound marker
	// through it.
	prop traffic.Propagator
	svr  *grpc.Server
}

// NewSimpleGrpcServer creates a SimpleGrpcServer from ${spring.grpc.server}
// configuration. Inbound inbound protection (rate-limit / breaker) is built
// inside buildResilienceInterceptors from the injected mgr. User-contributed
// interceptors arrive as bean collections: an application gs.Provides each
// interceptor and exports it As the interceptor type, and the container injects
// every one of them here — per-container, so two containers in one process carry
// independent stacks. Both collections are nullable, so an application that
// contributes no interceptor of its own still starts. center is the governance
// center bean, captured here and its authorities reused by the inbound and
// fault interceptors (nil when governance is not imported). prop is the
// application's load-test convention bean, handed to the LoadTest interceptors
// (nil means go-spring's default).
func NewSimpleGrpcServer(cfg Config, reg ServiceRegister,
	userUnary []grpc.UnaryServerInterceptor, userStream []grpc.StreamServerInterceptor,
	center *governance.Center, prop traffic.Propagator) *SimpleGrpcServer {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	log.Debugf(context.Background(), log.TagAppDef, "grpc server created addr=%s", cfg.Addr)
	return &SimpleGrpcServer{
		cfg:        cfg,
		reg:        reg,
		userUnary:  userUnary,
		userStream: userStream,
		center:     center,
		prop:       prop,
	}
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
	// User-contributed interceptors (bean collections injected at construction)
	// are outermost (first in the chain), mirroring starter-gin's
	// EngineMiddleware: an app guard sees the request before the built-in stack
	// and can short-circuit before any work is observed. Built-ins follow:
	// LoadTest, Tracing, Metrics, Resilience, then the handler.
	unary = append(unary, s.userUnary...)
	stream = append(stream, s.userStream...)
	// LoadTest identification is outermost of the built-ins so the marker is on
	// the context before tracing, metrics, resilience or the handler run, letting
	// every downstream layer branch on the injected propagator's IsLoadTest(ctx).
	// A no-op when the inbound metadata lacks the marker key.
	if s.cfg.LoadTest.Enabled {
		unary = append(unary, LoadTestUnaryInterceptor(s.prop))
		stream = append(stream, LoadTestStreamInterceptor(s.prop))
	}
	if s.cfg.Observer.Tracing.Enabled {
		unary = append(unary, TracingUnaryInterceptor())
		stream = append(stream, TracingStreamInterceptor())
	}
	// The access log is installed unconditionally, unlike the two above: it is
	// the one signal the RPC family requires of every member, so a config change
	// must not be able to remove it. It sits just inside tracing (the line then
	// carries the span's trace_id) and outside inbound, fault injection and
	// recovery (so it reports what the caller actually got).
	unary = append(unary, AccessLogUnaryInterceptor())
	stream = append(stream, AccessLogStreamInterceptor())
	if s.cfg.Observer.Metrics.Enabled {
		unary = append(unary, MetricsUnaryInterceptor())
		stream = append(stream, MetricsStreamInterceptor())
	}
	if ropts, ok := s.buildResilienceInterceptors(); ok {
		unary = append(unary, ropts.unary)
	}
	// Fault injection (always installed), innermost so an injected error flows
	// back through tracing/metrics/resilience and is observed. The interceptors
	// capture the injected injector bean once (nil-safe: a transparent
	// pass-through when governance is not imported), so fault can be hot-toggled
	// at runtime without a restart — the center swaps the injector's config in
	// place.
	unary = append(unary, FaultUnaryInterceptor(s.center.Fault()))
	stream = append(stream, FaultStreamInterceptor(s.center.Fault()))
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
	// The listening address rides on the context: every line this server logs
	// about its own lifecycle carries it without repeating it.
	ctx = log.WithFields(ctx, log.String("addr", s.cfg.Addr))

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
		log.Error(ctx, log.TagAppDef,
			log.Err(err),
			log.Msg("grpc server failed to listen"))
		return errutil.Explain(err, "failed to listen on %s", s.cfg.Addr)
	}
	<-sig.TriggerAndWait()
	log.Info(ctx, log.TagAppDef, log.Msg("grpc server starting"))
	if err = s.svr.Serve(listener); err != nil {
		log.Error(ctx, log.TagAppDef,
			log.Err(err),
			log.Msg("grpc server failed"))
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	}
	return nil
}

// Stop gracefully stops the underlying gRPC server. grpc's GracefulStop
// takes no context, so ctx only tags the shutdown log.
func (s *SimpleGrpcServer) Stop(ctx context.Context) error {
	log.Info(ctx, log.TagAppDef,
		log.String("addr", s.cfg.Addr),
		log.Msg("grpc server shutting down"))
	s.svr.GracefulStop()
	return nil
}
