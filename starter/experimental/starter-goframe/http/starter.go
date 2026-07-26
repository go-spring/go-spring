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

// Package StarterGoFrameHTTP integrates goframe's *ghttp.Server into the
// Go-Spring server lifecycle. Import it for the side effect and provide a
// ServiceRegister bean to attach routes:
//
//	import _ "go-spring.org/starter-goframe/http"
//
// Configuration is bound from the ${spring.goframe.http.server} prefix.
package StarterGoFrameHTTP

import (
	"context"

	otelmetric "github.com/gogf/gf/contrib/metric/otelmetric/v2"
	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/gsvc"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go.opentelemetry.io/otel/exporters/prometheus"

	// Side-effect import: installs the goframe (glog) -> go-spring log bridge
	// (see internal/logger). The bridge self-installs via init(), so importing
	// this package routes goframe's own logs into the application's go-spring
	// log pipeline before g.Server(name) emits its first lifecycle line.
	_ "go-spring.org/starter-goframe/internal/logger"
)

var goframeHTTPTag = log.RegisterAppTag("goframe_http", "starter")

func init() {
	gs.Provide(NewHTTPServer, gs.IndexArg(0, gs.TagArg("${spring.goframe.http.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.goframe.http.server.address"))
}

// ServiceRegister binds business routes onto the response-wrapping router group
// that HTTPServer sets up. This function type keeps the adapter
// service-agnostic: the adapter owns the server, the optional /metrics endpoint
// and the response-envelope middleware, while each service supplies its own
// controllers via this bean. The param is the *ghttp.RouterGroup (not the raw
// server) on purpose: business controllers must be bound inside the
// MiddlewareHandlerResponse group, whereas /metrics must stay at the server
// root (see NewHTTPServer).
type ServiceRegister func(group *ghttp.RouterGroup)

// Config binds goframe HTTP server settings from ${spring.goframe.http.server}.
//
// Observability note: tracing is deferred to starter-otel. ghttp
// auto-instruments every request off the global OpenTelemetry TracerProvider,
// so importing starter-otel lights up spans with no per-server config; without
// it, the global no-op provider is used. Metrics stay goframe-native (a
// Prometheus pull endpoint served by this same server) because that is separate
// from starter-otel's push pipeline; they are on by default.
type Config struct {
	Name    string `value:"${name:=goframe}"`
	Address string `value:"${address}"`

	// Registry publishes the server into etcd for discovery. Leave etcd empty
	// (the default) for a plain server with no registration — ghttp reads
	// gsvc.GetRegistry() at construction, so an unset registry means no
	// Register/Deregister happens.
	Registry struct {
		Etcd string `value:"${etcd:=}"`
	} `value:"${registry}"`

	// Metrics exposes goframe's native OTel Prometheus (pull) endpoint on this
	// same server. On by default so observability works out of the box. It
	// cannot be unified with starter-otel's metrics pipeline.
	Metrics struct {
		Enabled bool   `value:"${enabled:=true}"`
		Path    string `value:"${path:=/metrics}"`
	} `value:"${metrics}"`
}

// HTTPServer wraps a goframe *ghttp.Server so it satisfies gs.Server. The stock
// goframe pattern is a blocking s.Run() in main(); here the server implements
// gs.Server so Go-Spring drives startup and graceful shutdown alongside every
// other managed server.
type HTTPServer struct {
	svr        *ghttp.Server
	done       chan struct{}
	metricStop func(context.Context) error
}

// NewHTTPServer builds the goframe server from the Go-Spring-bound config,
// optionally registers an etcd-backed gsvc.Registry globally *before*
// g.Server(name) is called (ghttp.Server snapshots gsvc.GetRegistry() at
// construction time), optionally exposes a native Prometheus endpoint, and binds
// business routes via the injected ServiceRegister.
func NewHTTPServer(cfg Config, reg ServiceRegister) *HTTPServer {
	// Set the global registry first so g.Server(name) picks it up as its
	// registrar. Ordering matters: see ghttp/ghttp_server.go, where the
	// registrar field is read at server construction. Skipped when unconfigured.
	if cfg.Registry.Etcd != "" {
		gsvc.SetRegistry(etcdreg.New(cfg.Registry.Etcd))
	}

	log.Debugf(context.Background(), goframeHTTPTag, "goframe http server created name=%s address=%s registry=%s metrics=%v",
		cfg.Name, cfg.Address, cfg.Registry.Etcd, cfg.Metrics.Enabled)

	s := &HTTPServer{done: make(chan struct{})}

	svr := g.Server(cfg.Name)
	svr.SetAddr(cfg.Address)

	if cfg.Metrics.Enabled {
		s.initMetrics(svr, cfg)
	}

	// Business routes go inside the response-wrapping group; /metrics (bound in
	// initMetrics) stays at the server root so the Prometheus exposition is not
	// wrapped in goframe's JSON envelope.
	svr.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		reg(group)
	})
	s.svr = svr
	return s
}

// initMetrics wires goframe's native OTel metrics: a Prometheus (pull) exporter
// feeds a MeterProvider set as global, and the PrometheusHandler is bound at the
// server root under cfg.Metrics.Path — outside the response-wrapping group so the
// exposition stays valid Prometheus text.
func (s *HTTPServer) initMetrics(svr *ghttp.Server, cfg Config) {
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
	svr.BindHandler(cfg.Metrics.Path, otelmetric.PrometheusHandler)
}

// Run starts serving once Go-Spring signals readiness. goframe's Start() is
// non-blocking (it listens in a background goroutine and, when a registry is
// set, registers the service into etcd), so Run blocks on `done` until Stop is
// called, keeping the server bean alive for the container.
func (s *HTTPServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	log.Infof(ctx, goframeHTTPTag, "goframe http server starting")
	if err := s.svr.Start(); err != nil {
		log.Errorf(ctx, goframeHTTPTag, "goframe http server start failed: %v", err)
		return err
	}
	<-s.done
	return nil
}

// Stop gracefully shuts down the goframe server (which also deregisters from
// etcd when a registry is set), flushes the metric provider if any, and unblocks
// Run.
func (s *HTTPServer) Stop() error {
	log.Infof(context.Background(), goframeHTTPTag, "goframe http server shutting down")
	err := s.svr.Shutdown()
	if s.metricStop != nil {
		_ = s.metricStop(context.Background())
	}
	close(s.done)
	return err
}
