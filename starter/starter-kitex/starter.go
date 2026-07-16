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
	"os"
	"path/filepath"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	prometheus "github.com/kitex-contrib/monitor-prometheus"
	logrusadapter "github.com/kitex-contrib/obs-opentelemetry/logging/logrus"
	"github.com/kitex-contrib/obs-opentelemetry/provider"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/sirupsen/logrus"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Server side: gated on a ServiceRegister bean — no service to expose means
	// no server, so client-only apps are never forced to stand one up. Config is
	// read from the ${spring.kitex.server} prefix.
	enableSimpleKitexServer := gs.OnProperty("spring.kitex.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleKitexServer, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(
			NewSimpleKitexServer,
			gs.IndexArg(0, gs.TagArg("${spring.kitex.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister binds a service handler onto a raw Kitex server.Server. This
// function type keeps SimpleKitexServer service-agnostic: it drives the
// lifecycle while each service supplies its own register bean, typically
// wrapping the generated xxxservice.RegisterService.
type ServiceRegister func(svr server.Server) error

// Config defines Kitex server configuration, bound from ${spring.kitex.server}.
type Config struct {
	Addr        string `value:"${addr:=:8888}"`
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

	// Observability is opt-in and wired here rather than in each service so a
	// provider only edits conf/app.properties to light up the three pillars.
	// Kitex has no single "SetUp" like dubbo-go/go-zero, so we compose its
	// native kitex-contrib pieces: an OTel tracing suite, a self-hosting
	// Prometheus scrape endpoint, and a trace-correlated klog adapter.
	Tracing TracingCfg `value:"${tracing}"`
	Metrics MetricsCfg `value:"${metrics}"`
	Logging LoggingCfg `value:"${logging}"`
}

// TracingCfg configures OTel tracing under ${spring.kitex.server.tracing}. When
// enabled, spans are exported over OTLP/gRPC to Endpoint (a Jaeger/collector)
// and a tracing.NewServerSuite() is installed on the server. Metrics are
// intentionally left to MetricsCfg (Prometheus), so the OTel meter is disabled.
type TracingCfg struct {
	Enable   bool   `value:"${enable:=false}"`
	Endpoint string `value:"${endpoint:=127.0.0.1:4317}"`
	Insecure bool   `value:"${insecure:=true}"`
}

// MetricsCfg configures Prometheus metrics under ${spring.kitex.server.metrics}.
// When enabled, monitor-prometheus stands up its own HTTP server on Port serving
// Path, independent of the (usually disabled) built-in spring.http.server, for a
// Prometheus instance to scrape.
type MetricsCfg struct {
	Enable bool   `value:"${enable:=false}"`
	Port   int    `value:"${port:=9090}"`
	Path   string `value:"${path:=/metrics}"`
}

// LoggingCfg configures kitex's global klog under ${spring.kitex.server.logging}.
// When enabled, klog is backed by the obs-opentelemetry logrus adapter emitting
// JSON lines (with trace_id/span_id injected from the request context) to
// Dir/File, so a log shipper like Promtail can tail and correlate them.
type LoggingCfg struct {
	Enable bool   `value:"${enable:=false}"`
	Dir    string `value:"${dir:=../logs}"`
	File   string `value:"${file:=provider.log}"`
	Level  string `value:"${level:=info}"`
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

	// otelProvider is the OTel SDK provider created when tracing is enabled; it
	// owns the span exporter and is shut down in Stop to flush pending spans.
	otelProvider provider.OtelProvider
	// logFile is the klog output file opened when logging is enabled; closed in
	// Stop so the final buffered lines are flushed to disk.
	logFile *os.File
}

// NewSimpleKitexServer creates a SimpleKitexServer from ${spring.kitex.server}
// config and the registered ServiceRegister bean.
func NewSimpleKitexServer(cfg Config, reg ServiceRegister) *SimpleKitexServer {
	return &SimpleKitexServer{cfg: cfg, reg: reg, done: make(chan struct{})}
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

	// Observability is layered on last so a provider lights up the three pillars
	// purely from conf/app.properties. Logging must be set up before serving so
	// the very first request is captured; tracing/metrics contribute a suite and
	// a stats tracer to the option set below.
	if err = s.setupLogging(); err != nil {
		return err
	}
	if s.cfg.Tracing.Enable {
		popts := []provider.Option{
			provider.WithServiceName(s.cfg.ServiceName),
			provider.WithExportEndpoint(s.cfg.Tracing.Endpoint),
			// Metrics travel through Prometheus (see below), so the OTel meter is
			// disabled to avoid a second, redundant metrics pipeline.
			provider.WithEnableMetrics(false),
		}
		if s.cfg.Tracing.Insecure {
			popts = append(popts, provider.WithInsecure())
		}
		s.otelProvider = provider.NewOpenTelemetryProvider(popts...)
		opts = append(opts, server.WithSuite(tracing.NewServerSuite()))
	}
	if s.cfg.Metrics.Enable {
		// NewServerTracer stands up its own HTTP server on this addr serving the
		// metrics path, independent of the built-in spring.http.server.
		metricsAddr := fmt.Sprintf(":%d", s.cfg.Metrics.Port)
		opts = append(opts, server.WithTracer(prometheus.NewServerTracer(metricsAddr, s.cfg.Metrics.Path)))
	}

	s.svr = server.NewServer(opts...)
	if err = s.reg(s.svr); err != nil {
		return errutil.Explain(err, "failed to register service")
	}

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// Run binds the listener, registers into etcd and then blocks.
		errCh <- s.svr.Run()
	}()

	select {
	case err = <-errCh:
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	case <-s.done:
		return nil
	}
}

// Stop gracefully stops the underlying Kitex server, deregistering it from
// etcd, and signals Run to return so Go-Spring can complete shutdown. It also
// tears down the observability resources set up in Run: the OTel provider is
// shut down to flush pending spans, and the klog output file is closed.
func (s *SimpleKitexServer) Stop() error {
	err := s.svr.Stop()
	if s.otelProvider != nil {
		_ = s.otelProvider.Shutdown(context.Background())
	}
	if s.logFile != nil {
		_ = s.logFile.Close()
	}
	close(s.done)
	return err
}

// setupLogging backs kitex's global klog with the obs-opentelemetry logrus
// adapter when logging is enabled. The adapter injects trace_id/span_id from the
// request context on the CtxXxx calls, and a JSON formatter plus a file sink make
// the output ready for a log shipper to tail and correlate with traces. It is a
// no-op when logging is disabled, leaving klog's default stderr logger in place.
func (s *SimpleKitexServer) setupLogging() error {
	if !s.cfg.Logging.Enable {
		return nil
	}
	if err := os.MkdirAll(s.cfg.Logging.Dir, 0o755); err != nil {
		return errutil.Explain(err, "failed to create log dir %s", s.cfg.Logging.Dir)
	}
	path := filepath.Join(s.cfg.Logging.Dir, s.cfg.Logging.File)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return errutil.Explain(err, "failed to open log file %s", path)
	}
	s.logFile = f

	logger := logrusadapter.NewLogger()
	logger.Logger().SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(f)
	logger.SetLevel(parseKlogLevel(s.cfg.Logging.Level))
	klog.SetLogger(logger)
	return nil
}

// parseKlogLevel maps a config level string to a klog.Level, defaulting to Info
// for empty or unrecognized values so a typo never silences logging entirely.
func parseKlogLevel(level string) klog.Level {
	switch level {
	case "trace":
		return klog.LevelTrace
	case "debug":
		return klog.LevelDebug
	case "notice":
		return klog.LevelNotice
	case "warn":
		return klog.LevelWarn
	case "error":
		return klog.LevelError
	case "fatal":
		return klog.LevelFatal
	default:
		return klog.LevelInfo
	}
}
