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

// Package main hosts the provider binary. server.go adapts goframe's
// *ghttp.Server to the Go-Spring server lifecycle. In a stock goframe project
// this wiring lives in internal/cmd: g.Server() -> s.Group(...) route binding
// -> s.Run(). Here it is expressed as a gs.Server bean so the container drives
// startup and graceful shutdown, and it wires goframe's built-in etcd registry
// so the provider publishes itself into etcd instead of being reached via a
// hard-coded host:port.
package main

import (
	"context"

	otelmetric "github.com/gogf/gf/contrib/metric/otelmetric/v2"
	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	otlphttp "github.com/gogf/gf/contrib/trace/otlphttp/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/gsvc"
	"go-spring.org/spring/gs"
	"go.opentelemetry.io/otel/exporters/prometheus"
)

func init() {
	// The goframe *ghttp.Server, exported as a gs.Server so the Go-Spring
	// lifecycle starts and stops it. Config is bound from the "${goframe}"
	// prefix. This replaces the g.Server() + s.Run() block goframe emits in
	// internal/cmd, which blocked in main() and owned the process.
	gs.Provide(NewGoFrameServer, gs.IndexArg(0, gs.TagArg("${goframe}"))).
		Export(gs.As[gs.Server]())
}

// Config holds the goframe HTTP server settings.
//
// In a stock goframe project these come from manifest/config/config.yaml and
// are loaded implicitly by g.Server() via g.Cfg(). Here they are bound from
// Go-Spring properties (see conf/app.properties) under the "${goframe}" prefix
// using `value` tags, so the config source moves out of goframe's own loader.
//
// Name and RegistryAddr are what turn the earlier single-process example into a
// real provider/consumer split: the provider registers itself into etcd under
// Name, and the consumer resolves that same name from etcd instead of dialing
// a hard-coded host:port.
//
// Beyond those, this example surfaces the three observability pillars using
// goframe's own OTel integration (not go-spring's log/metric): tracing via
// contrib/trace/otlphttp, metrics via contrib/metric/otelmetric exposed as a
// Prometheus scrape endpoint, and structured logging bridged from glog into
// go-spring's log module (see logbridge.go — one JSON pipeline for framework
// AND business logs). We only instrument the provider — the consumer stays a
// bare client, matching the dubbo-go / go-zero examples. Each field carries an
// explicit default so the plain smoke test still runs when no observability
// backend is listening (the trace exporter just fails to connect in the
// background).
type Config struct {
	Address      string `value:"${address:=:8000}"`
	Name         string `value:"${name:=goframe.hello}"`
	RegistryAddr string `value:"${registry.etcd:=127.0.0.1:2379}"`

	// Tracing: goframe's contrib/trace/otlphttp exports spans to Jaeger over
	// OTLP/HTTP (Jaeger's 4318). ghttp auto-instruments every request once the
	// global tracer provider is set — no middleware wiring needed. OTLP/HTTP is
	// used (not gRPC) because otlphttp hardcodes WithInsecure(), matching the
	// dubbo-go example's reasoning for talking to a plaintext collector.
	TracingEndpoint string `value:"${tracing.endpoint:=127.0.0.1:4318}"`
	TracingPath     string `value:"${tracing.path:=/v1/traces}"`

	// Metrics: otelmetric builds a MeterProvider fed by a Prometheus (pull)
	// exporter; the endpoint is served on the same ghttp server under
	// MetricsPath, bound outside the response-wrapping group so the exposition
	// stays valid Prometheus text. WithBuiltInMetrics() adds Go runtime metrics.
	MetricsPath string `value:"${metrics.path:=/metrics}"`

	// Logging: goframe's glog default handler is redirected into go-spring's
	// log module by installGoFrameLogBridge (see logbridge.go). The actual
	// file sink is the root FileLogger declared in conf/app.properties, which
	// Promtail tails into Loki. No goframe-side log path config is kept here
	// — the bridge suppresses glog's own output, so any glog file wiring
	// would either be dead config or a duplicate writer.
}

// GoFrameServer wraps a goframe *ghttp.Server so it satisfies gs.Server.
type GoFrameServer struct {
	svr        *ghttp.Server
	done       chan struct{}
	traceStop  func(context.Context)
	metricStop func(context.Context) error
}

// NewGoFrameServer builds the goframe server from the Go-Spring-bound config,
// registers an etcd-backed gsvc.Registry globally *before* g.Server(name) is
// called (ghttp.Server snapshots gsvc.GetRegistry() at construction time),
// wires the three observability pillars, and binds the HelloController (see
// handler.go). When Start is invoked later, ghttp will publish the service
// under cfg.Name into etcd; on Shutdown it deregisters itself.
func NewGoFrameServer(cfg Config) *GoFrameServer {
	// Route goframe's own glog logs into go-spring's log module (see
	// logbridge.go). Installed before g.Server(name) so ghttp lifecycle and
	// gsvc registration lines flow through the same pipeline as the business
	// logs. initObservability below deliberately no longer installs
	// glog.HandlerJson, because a per-logger Handlers slice would override the
	// bridge for g.Log() — see logbridge.go's install func for the rationale.
	installGoFrameLogBridge()

	// Set the global registry first so g.Server(name) picks it up as its
	// registrar. See ghttp/ghttp_server.go: `registrar: gsvc.GetRegistry()`
	// is read at server construction, so ordering matters.
	gsvc.SetRegistry(etcdreg.New(cfg.RegistryAddr))

	gsrv := &GoFrameServer{done: make(chan struct{})}
	gsrv.initObservability(cfg)

	s := g.Server(cfg.Name)
	s.SetAddr(cfg.Address)
	// /metrics is bound at the server root, NOT inside the group below: the
	// group's MiddlewareHandlerResponse wraps every response in goframe's JSON
	// envelope, which would corrupt the Prometheus exposition format.
	s.BindHandler(cfg.MetricsPath, otelmetric.PrometheusHandler)
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Bind(
			&HelloController{},
		)
	})
	gsrv.svr = s
	return gsrv
}

// initObservability wires goframe's native OTel integration for the three
// pillars. It sets process-global providers, so it must run before the server
// starts handling requests (ghttp reads the global tracer/meter provider when
// it instruments a request).
func (s *GoFrameServer) initObservability(cfg Config) {
	// Tracing: otlphttp.Init sets the global tracer + propagator and returns a
	// shutdown func (flushes the batch span processor). ghttp then instruments
	// every request automatically — no middleware call needed.
	shutdown, err := otlphttp.Init(cfg.Name, cfg.TracingEndpoint, cfg.TracingPath)
	if err != nil {
		g.Log().Fatalf(context.Background(), "init tracing: %+v", err)
	}
	s.traceStop = shutdown

	// Metrics: a Prometheus (pull) exporter feeds the MeterProvider. The
	// PrometheusHandler bound above serves this exporter's default registry.
	exporter, err := prometheus.New()
	if err != nil {
		g.Log().Fatalf(context.Background(), "init metrics exporter: %+v", err)
	}
	provider := otelmetric.MustProvider(
		otelmetric.WithReader(exporter),
		otelmetric.WithBuiltInMetrics(),
	)
	provider.SetAsGlobal()
	s.metricStop = provider.Shutdown

	// Logging: glog's default handler is already redirected into go-spring's
	// log module by installGoFrameLogBridge (called from NewGoFrameServer).
	// We deliberately do NOT call logger.SetHandlers(glog.HandlerJson) here:
	// per-logger Handlers take precedence over the package default (see
	// glog_logger.go), so installing HandlerJson on g.Log() would defeat the
	// bridge and split framework logs from business logs. The file sink is
	// the root FileLogger declared in conf/app.properties.
	_ = cfg
}

// Run starts serving once Go-Spring signals readiness. goframe's Start() is
// non-blocking (it listens in a background goroutine and registers the service
// into etcd), so Run blocks on `done` until Stop is called, keeping the server
// bean alive for the container.
func (s *GoFrameServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	if err := s.svr.Start(); err != nil {
		return err
	}
	<-s.done
	return nil
}

// Stop gracefully shuts down the goframe server (which also deregisters from
// etcd), flushes the trace/metric providers, and unblocks Run. This replaces
// the process-owned signal handling that s.Run() would otherwise install.
func (s *GoFrameServer) Stop() error {
	ctx := context.Background()
	err := s.svr.Shutdown()
	if s.traceStop != nil {
		s.traceStop(ctx)
	}
	if s.metricStop != nil {
		_ = s.metricStop(ctx)
	}
	close(s.done)
	return err
}
