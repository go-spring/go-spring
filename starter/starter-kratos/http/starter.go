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

// Package StarterKratosHttp integrates kratos' HTTP transport server into the
// Go-Spring server lifecycle. Import it for the side effect and provide a
// ServiceRegister bean to bind services:
//
//	import _ "go-spring.org/starter-kratos/http"
//
// Configuration is bound from the ${spring.kratos.http.server} prefix. Leave
// Etcd.Addr empty for a plain direct-connect server, or set it to publish the
// service into etcd for discovery under Name. Tracing is deferred to the ambient
// global OTel provider (import starter-otel to light it up); request metrics are
// opt-in via Metrics.Enable and feed the same global meter.
package StarterKratosHttp

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2"
	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	kmetrics "github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.opentelemetry.io/otel"

	"go-spring.org/starter-kratos/internal/logger"
)

func init() {
	// Importing the starter is the opt-in; the module still guards on
	// spring.kratos.http.server.enabled (default true). The server only
	// materializes when the application supplies a ServiceRegister bean, keeping
	// HttpServer independent of any concrete proto-generated service.
	enabled := gs.OnProperty("spring.kratos.http.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enabled, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(NewHttpServer, gs.IndexArg(0, gs.TagArg("${spring.kratos.http.server}"))).
			Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister binds services onto a kratos HTTP transport server. Extracting
// registration behind this function type keeps HttpServer service-agnostic: it
// drives the lifecycle while each service supplies its own register bean,
// typically wrapping the generated xxx.RegisterXxxHTTPServer.
type ServiceRegister func(hs *khttp.Server) error

// Config binds kratos HTTP server + etcd registry configuration from
// ${spring.kratos.http.server}.
type Config struct {
	Name    string        `value:"${name:=kratos-http}"`
	Network string        `value:"${network:=}"`
	Addr    string        `value:"${addr:=0.0.0.0:8000}"`
	Timeout time.Duration `value:"${timeout:=1s}"`

	// Etcd service discovery. Empty Addr disables registration (direct-connect).
	Etcd struct {
		Addr string `value:"${addr:=}"`
	} `value:"${etcd}"`

	// Metrics is opt-in. When enabled, the kratos metrics middleware records the
	// request counter and latency histogram into the process-global OTel meter —
	// so the actual exporter and scrape endpoint are owned by starter-otel, not
	// by this starter. Tracing needs no flag: tracing.Server() always reads the
	// global tracer (a no-op provider when starter-otel is absent).
	Metrics struct {
		Enable bool `value:"${enable:=false}"`
	} `value:"${metrics}"`
}

// HttpServer adapts a kratos.App wrapping a single HTTP transport server to the
// Go-Spring server lifecycle. The App implements startup, etcd registration and
// graceful shutdown; Go-Spring drives it alongside every other managed server.
type HttpServer struct {
	cfg  Config
	reg  ServiceRegister
	log  klog.Logger
	app  *kratos.App
	done chan struct{}
}

// NewHttpServer builds an HttpServer from ${spring.kratos.http.server} config and
// the registered ServiceRegister bean. The kratos logger bridges framework logs
// into go-spring's log module (see internal/logger).
func NewHttpServer(cfg Config, reg ServiceRegister) *HttpServer {
	return &HttpServer{cfg: cfg, reg: reg, log: logger.NewLogger(), done: make(chan struct{})}
}

// Run builds the kratos HTTP transport server, composes it into a kratos.App
// together with an optional etcd Registrar, and starts serving once Go-Spring
// signals readiness. kratos.App.Run publishes the service into etcd (when a
// registrar is configured) and blocks until Stop is called, so it runs in a
// goroutine while Run parks on the done channel; Stop closes done to hand control
// back to Go-Spring after tearing the App down.
func (s *HttpServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	// Middleware order: recovery outermost, then tracing (starts the span), then
	// metrics (records under the active span/context). tracing.Server() reads the
	// global OTel provider installed by starter-otel; absent that it is a no-op.
	mw := []middleware.Middleware{recovery.Recovery(), tracing.Server()}
	if s.cfg.Metrics.Enable {
		meter := otel.Meter(s.cfg.Name)
		requests, err := kmetrics.DefaultRequestsCounter(meter, "server_requests_code_total")
		if err != nil {
			return errutil.Explain(err, "failed to create requests counter")
		}
		seconds, err := kmetrics.DefaultSecondsHistogram(meter, "server_requests_seconds")
		if err != nil {
			return errutil.Explain(err, "failed to create seconds histogram")
		}
		mw = append(mw, kmetrics.Server(kmetrics.WithRequests(requests), kmetrics.WithSeconds(seconds)))
	}

	opts := []khttp.ServerOption{khttp.Middleware(mw...)}
	if s.cfg.Network != "" {
		opts = append(opts, khttp.Network(s.cfg.Network))
	}
	if s.cfg.Addr != "" {
		opts = append(opts, khttp.Address(s.cfg.Addr))
	}
	if s.cfg.Timeout != 0 {
		opts = append(opts, khttp.Timeout(s.cfg.Timeout))
	}
	httpSrv := khttp.NewServer(opts...)

	if err := s.reg(httpSrv); err != nil {
		return errutil.Explain(err, "failed to register kratos http service")
	}

	appOpts := []kratos.Option{
		kratos.Name(s.cfg.Name),
		kratos.Logger(s.log),
		kratos.Server(httpSrv),
	}
	// Registry turns a direct-connect setup into a real service: on Run the App
	// publishes {Name, endpoints} into etcd and Deregisters on stop. Opt-in:
	// leaving Etcd.Addr empty runs a registry-free server reached by host:port.
	if s.cfg.Etcd.Addr != "" {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{s.cfg.Etcd.Addr},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			return errutil.Explain(err, "failed to create etcd client for %s", s.cfg.Etcd.Addr)
		}
		appOpts = append(appOpts, kratos.Registrar(etcd.New(cli)))
	}
	s.app = kratos.New(appOpts...)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.app.Run()
	}()

	select {
	case err := <-errCh:
		return errutil.Explain(err, "kratos http app exited with error")
	case <-s.done:
		return s.app.Stop()
	}
}

// Stop signals Run to tear down the kratos.App so Go-Spring can complete its
// shutdown sequence.
func (s *HttpServer) Stop() error {
	close(s.done)
	return nil
}
