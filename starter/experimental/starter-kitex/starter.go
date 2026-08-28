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

package StarterKitex

import (
	"context"
	"fmt"
	"net"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	prometheus "github.com/kitex-contrib/monitor-prometheus"
	"github.com/kitex-contrib/obs-opentelemetry/provider"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	etcd "github.com/kitex-contrib/registry-etcd"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"

	// Side-effect import: installs the kitex -> go-spring log bridge (see
	// internal/logger). The bridge self-installs via init(), so no symbols are
	// referenced here - importing this package is what redirects kitex' own
	// logs into the application's go-spring log pipeline.
	_ "go-spring.org/starter-kitex/internal/logger"
)

func init() {
	gs.Provide(
		NewSimpleKitexServer,
		gs.IndexArg(0, gs.TagArg("${spring.kitex.server}")),
	).Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.kitex.server.addr"))
}

// ServiceRegister binds a service handler onto a raw Kitex server.Server. This
// function type keeps SimpleKitexServer service-agnostic: it drives the
// lifecycle while each service supplies its own register bean, typically
// wrapping the generated xxxservice.RegisterService.
type ServiceRegister func(svr server.Server) error

// Config defines Kitex server configuration, bound from ${spring.kitex.server}.
type Config struct {
	Addr        string `value:"${addr}"`
	ServiceName string `value:"${service.name:=kitex}"`

	// RegistryAddr is the etcd registry address. Empty (the default) runs a
	// registry-free server reached directly by host:port; set it to publish the
	// service into etcd for discovery under ServiceName.
	RegistryAddr string `value:"${registry.etcd:=}"`

	// CompatibleUnaryMiddleware appends server.WithCompatibleMiddlewareForUnary.
	// Kitex's thrift codegen adds this in its generated NewServer, but the
	// protobuf codegen does not — so enable it for thrift services and leave it
	// off for protobuf/gRPC ones.
	CompatibleUnaryMiddleware bool `value:"${compatible-unary-middleware:=false}"`

	// Observability is global-first and wired here rather than in each service
	// so a provider only edits conf/app.properties to light up metrics and
	// tracing. When go-spring's unified observability (starter-otel) has
	// installed the process-global OTel providers, the kitex tracing suite
	// simply attaches to them — kitex never builds a second pipeline. Only
	// when no global pipeline exists does the starter fall back to building
	// its own provider, preserving the zero-config experience for kitex-only
	// setups. Kitex' own klog is bridged into go-spring's log module
	// unconditionally (see internal/logger).
	Tracing TracingCfg `value:"${tracing}"`
	Metrics MetricsCfg `value:"${metrics}"`
}

// TracingCfg configures OTel tracing under ${spring.kitex.server.tracing}. On
// by default. Global-first: when a real global TracerProvider is installed
// (starter-otel), the kitex suite attaches to it and the remaining fields are
// ignored — the global pipeline owns exporters and sampling. Without a global
// pipeline the starter builds its own provider exporting OTLP/gRPC to Endpoint
// (see globalTracingActive for how the decision is made).
type TracingCfg struct {
	Enable   bool   `value:"${enable:=true}"`
	Endpoint string `value:"${endpoint:=127.0.0.1:4317}"`
	Insecure bool   `value:"${insecure:=true}"`
}

// MetricsCfg configures Prometheus metrics under ${spring.kitex.server.metrics}.
// Enable is on by default, but the dedicated scrape server only starts when
// Port is explicitly configured — server ports are never defaulted here (the
// "server ports must be explicitly configured" repo convention). With a global
// pipeline present and no Port set, kitex RPC metrics (kitex.server.duration
// histogram recorded by the tracing suite) ride the global metrics exporter
// instead, so no dedicated endpoint is needed.
type MetricsCfg struct {
	Enable bool   `value:"${enable:=true}"`
	Port   int    `value:"${port:=0}"`
	Path   string `value:"${path:=/metrics}"`
}

// SimpleKitexServer adapts a Kitex server.Server to the Go-Spring server
// lifecycle. The scaffold ran svr.Run() directly from main(), which blocks and
// owns the process. Here the server implements gs.Server so Go-Spring drives
// startup and graceful shutdown alongside every other managed server.
type SimpleKitexServer struct {
	cfg  Config
	reg  ServiceRegister
	svr  server.Server
	done chan struct{}

	// otelProvider is the local OTel SDK provider created by the tracing
	// fallback when no global pipeline is present (see observabilityOptions);
	// nil when kitex attaches to starter-otel's global pipeline. It owns the
	// span exporter and is shut down in Stop to flush pending spans.
	otelProvider provider.OtelProvider
}

// NewSimpleKitexServer creates a SimpleKitexServer from ${spring.kitex.server}
// config and the registered ServiceRegister bean.
func NewSimpleKitexServer(cfg Config, reg ServiceRegister) *SimpleKitexServer {
	log.Debugf(context.Background(), log.TagAppDef, "kitex server created addr=%s service=%s", cfg.Addr, cfg.ServiceName)
	return &SimpleKitexServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// probeTracer names the tracer used to detect a live global OTel pipeline.
const probeTracer = "go-spring.org/starter-kitex/probe"

// globalTracingActive reports whether a real (recording) TracerProvider has
// been installed as the OTel process global — i.e. whether go-spring's unified
// observability (starter-otel, or any SDK-based provider) owns the pipeline.
// Without one, otel.Tracer returns the no-op provider whose spans never
// record, so IsRecording is false.
//
// Caveat: a global provider configured with an always-off sampler also yields
// non-recording spans, so the probe reports "no global pipeline" there and the
// starter would fall back to its local provider. That configuration drops all
// spans by design, so no double-pipeline harm results; see USAGE §4.2.
func globalTracingActive() bool {
	_, span := otel.Tracer(probeTracer).Start(context.Background(), "kitex-global-pipeline-probe")
	recording := span.IsRecording()
	span.End()
	return recording
}

// observabilityOptions builds the kitex server options for tracing and metrics
// under the global-first rule, plus the local OTel provider it may own.
//
// Tracing (cfg.Tracing.Enable, default true):
//
//	Global pipeline present  → attach tracing.NewServerSuite() only. The suite
//	                           reads the OTel globals (provider + propagator),
//	                           so kitex spans and the kitex.server.duration
//	                           metric flow into the unified pipeline. No
//	                           provider is created and the TracingCfg endpoint
//	                           fields are ignored.
//	No global pipeline       → fall back to building a local provider via
//	                           provider.NewOpenTelemetryProvider (exporting
//	                           OTLP/gRPC to TracingCfg.Endpoint) so a kitex-only
//	                           setup lights up spans with zero extra config,
//	                           and log a hint toward starter-otel.
//
// Metrics (cfg.Metrics.Enable, default true):
//
//	Port explicitly set      → stand up monitor-prometheus' dedicated scrape
//	                           server on that port (explicit opt-in wins over
//	                           the global pipeline, for a dedicated kitex
//	                           /metrics next to it).
//	Port unset (the default) → no dedicated endpoint. With a global pipeline
//	                           present, kitex RPC metrics ride it through the
//	                           tracing suite's otel meter. Without one, log
//	                           how to enable the local endpoint — never
//	                           default-bind a server port.
//
// The returned provider (nil unless the local fallback ran) is stored by the
// caller and shut down on Stop to flush pending spans.
func observabilityOptions(cfg Config) (opts []server.Option, localProvider provider.OtelProvider) {
	active := globalTracingActive()

	if cfg.Tracing.Enable {
		if active {
			log.Infof(context.Background(), log.TagAppDef,
				"kitex tracing attached to the global otel pipeline (starter-otel); kitex tracing.* endpoint keys are ignored")
		} else {
			popts := []provider.Option{
				provider.WithServiceName(cfg.ServiceName),
				provider.WithExportEndpoint(cfg.Tracing.Endpoint),
				// Metrics travel through Prometheus (see below), so the OTel meter
				// is disabled to avoid a second, redundant metrics pipeline.
				provider.WithEnableMetrics(false),
			}
			if cfg.Tracing.Insecure {
				popts = append(popts, provider.WithInsecure())
			}
			localProvider = provider.NewOpenTelemetryProvider(popts...)
			log.Infof(context.Background(), log.TagAppDef,
				"kitex tracing running on its own otel provider (endpoint=%s); import starter-otel for unified observability", cfg.Tracing.Endpoint)
		}
		opts = append(opts, server.WithSuite(tracing.NewServerSuite()))
	}

	if cfg.Metrics.Enable {
		switch {
		case cfg.Metrics.Port > 0:
			// NewServerTracer stands up its own HTTP server on this addr serving
			// the metrics path, independent of the built-in spring.http.server.
			// Only started on an explicitly configured port; the library calls
			// log.Fatal on a bind failure, so the port must be operator-chosen.
			opts = append(opts, server.WithTracer(prometheus.NewServerTracer(
				fmt.Sprintf(":%d", cfg.Metrics.Port), cfg.Metrics.Path)))
		case active:
			log.Infof(context.Background(), log.TagAppDef,
				"kitex metrics ride the global otel pipeline (kitex.server.duration via the tracing suite); set metrics.port for a dedicated prometheus endpoint")
		default:
			log.Infof(context.Background(), log.TagAppDef,
				"kitex metrics disabled: no global otel pipeline and no metrics.port configured; set spring.kitex.server.metrics.port to start a dedicated prometheus endpoint")
		}
	}
	return opts, localProvider
}

// Run builds the Kitex server on the configured address and starts serving once
// Go-Spring signals readiness. Serving with a registry configured makes Kitex
// publish the provider's address into etcd under its service name; a consumer
// later resolves a live provider by the same name. server.Run blocks forever
// internally, so it runs in a goroutine while Run parks on the done channel;
// Stop closes done to hand control back to Go-Spring.
func (s *SimpleKitexServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	addr, err := net.ResolveTCPAddr("tcp", s.cfg.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to resolve addr %s", s.cfg.Addr)
	}

	// Build the raw Kitex server. This inlines what a generated
	// xxxservice.NewServer would do — construct the server and register the
	// service handler — so the adapter owns construction and only defers the
	// service-specific binding to the injected ServiceRegister.
	opts := []server.Option{
		server.WithServiceAddr(addr),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: s.cfg.ServiceName,
		}),
	}

	// Registry turns a direct-connect setup into a real service: on Run the
	// provider registers itself into etcd for discovery under ServiceName. It is
	// opt-in — leaving RegistryAddr empty runs a registry-free server that
	// clients reach directly by host:port.
	if s.cfg.RegistryAddr != "" {
		r, err := etcd.NewEtcdRegistry([]string{s.cfg.RegistryAddr})
		if err != nil {
			return errutil.Explain(err, "failed to create etcd registry")
		}
		opts = append(opts, server.WithRegistry(r))
	}

	if s.cfg.CompatibleUnaryMiddleware {
		opts = append(opts, server.WithCompatibleMiddlewareForUnary())
	}

	// Observability is layered on last so a provider lights up metrics and
	// tracing purely from conf/app.properties; see observabilityOptions for
	// the global-first pipeline rules.
	obsOpts, localProvider := observabilityOptions(s.cfg)
	if localProvider != nil {
		s.otelProvider = localProvider
	}
	opts = append(opts, obsOpts...)

	s.svr = server.NewServer(opts...)
	if err = s.reg(s.svr); err != nil {
		return errutil.Explain(err, "failed to register service")
	}

	<-sig.TriggerAndWait()

	log.Infof(ctx, log.TagAppDef, "kitex server starting on %s", s.cfg.Addr)
	errCh := make(chan error, 1)
	go func() {
		// Run binds the listener, registers into etcd and then blocks.
		errCh <- s.svr.Run()
	}()

	select {
	case err = <-errCh:
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "kitex server failed on %s: %v", s.cfg.Addr, err)
		}
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	case <-s.done:
		return nil
	}
}

// Stop gracefully stops the underlying Kitex server, deregistering it from
// etcd, and signals Run to return so Go-Spring can complete shutdown. It also
// shuts down the OTel provider set up in Run to flush pending spans.
func (s *SimpleKitexServer) Stop() error {
	return s.StopContext(context.Background())
}

// StopContext gracefully stops the underlying Kitex server, deregistering it
// from etcd, and signals Run to return so Go-Spring can complete shutdown. It
// also shuts down the OTel provider set up in Run to flush pending spans,
// threading the shutdown context into the provider's Shutdown. Kitex's Stop
// takes no context, so ctx is otherwise only used for logging.
func (s *SimpleKitexServer) StopContext(ctx context.Context) error {
	log.Infof(ctx, log.TagAppDef, "kitex server shutting down on %s", s.cfg.Addr)
	err := s.svr.Stop()
	if s.otelProvider != nil {
		_ = s.otelProvider.Shutdown(ctx)
	}
	close(s.done)
	return err
}
