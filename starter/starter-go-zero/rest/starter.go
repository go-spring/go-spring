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

// Package StarterGoZeroRest integrates go-zero's rest.Server (HTTP/API, and by
// extension WebSocket, which go-zero serves as an upgrade route on rest.Server)
// into the Go-Spring server lifecycle. Import it for the side effect and provide
// a HandlerRegister bean to attach routes:
//
//	import _ "go-spring.org/starter-go-zero/rest"
//
// Configuration is bound from the ${spring.go-zero.rest.server} prefix.
package StarterGoZeroRest

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/trace"
	"github.com/zeromicro/go-zero/rest"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"

	"go-spring.org/starter-go-zero/internal/logger"
)

func init() {
	// Importing the starter is the opt-in; the module still guards on
	// spring.go-zero.rest.server.enabled (default true) so it can be turned off
	// without dropping the import. The rest.Server only materializes when the
	// application supplies a HandlerRegister bean, keeping RestServer
	// service-agnostic — each service registers its own routes.
	enabled := gs.OnProperty("spring.go-zero.rest.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enabled, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(NewRestServer, gs.IndexArg(0, gs.TagArg("${spring.go-zero.rest.server}"))).
			Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[HandlerRegister]())
		return nil
	})
}

// HandlerRegister attaches handlers onto a *rest.Server. Extracting registration
// behind this function type keeps RestServer independent of any concrete route
// table; each service supplies its own register bean.
type HandlerRegister func(server *rest.Server)

// Config binds go-zero rest.Server configuration from
// ${spring.go-zero.rest.server}.
//
// Observability note: tracing is deferred to starter-otel by default. When
// Tracing.Disabled is true (the default) go-zero does NOT start its own trace
// agent, so the rest.Server trace middleware emits spans through whatever global
// OTel TracerProvider is installed — i.e. the one starter-otel sets up when
// imported, or a no-op otherwise. Flip Tracing.Disabled=false to use go-zero's
// native OTLP export instead. Metrics remain go-zero-native (Prometheus via
// DevServer) because go-zero does not emit OTel metrics; the DevServer is off by
// default and can be enabled independently.
type Config struct {
	Name string `value:"${name:=go-zero}"`
	Host string `value:"${host:=0.0.0.0}"`
	Port int    `value:"${port:=8888}"`

	// Tracing: defer to starter-otel by default (Disabled=true). Endpoint /
	// Sampler / Batcher only take effect when Disabled=false, i.e. go-zero
	// native OTLP export.
	Tracing struct {
		Disabled bool    `value:"${disabled:=true}"`
		Endpoint string  `value:"${endpoint:=}"`
		Sampler  float64 `value:"${sampler:=1.0}"`
		Batcher  string  `value:"${batcher:=otlpgrpc}"`
	} `value:"${tracing}"`

	// Metrics: go-zero's DevServer exposes a Prometheus /metrics endpoint on its
	// own port. Off by default — enable only if scraping go-zero's native
	// Prometheus registry.
	Metrics struct {
		Enabled bool   `value:"${enabled:=false}"`
		Port    int    `value:"${port:=6060}"`
		Path    string `value:"${path:=/metrics}"`
	} `value:"${metrics}"`

	// Log level is the only logx field still honoured after the bridge is
	// installed: logx filters by level before delegating to the bridge Writer.
	Log struct {
		Level string `value:"${level:=info}"`
	} `value:"${log}"`
}

// RestServer adapts a go-zero rest.Server to the Go-Spring server lifecycle. The
// stock go-zero pattern is a blocking srv.Start() in main(); here the server
// implements gs.Server so Go-Spring drives startup and graceful shutdown
// alongside every other managed server.
type RestServer struct {
	cfg  Config
	reg  HandlerRegister
	svr  *rest.Server
	done chan struct{}
}

// NewRestServer builds a RestServer from ${spring.go-zero.rest.server} config
// and the registered HandlerRegister bean.
func NewRestServer(cfg Config, reg HandlerRegister) *RestServer {
	return &RestServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// Run builds the rest.Server, lets the HandlerRegister bean attach routes, then
// serves once Go-Spring signals readiness. rest.Server.Start blocks internally,
// so it runs in a goroutine while Run parks on the done channel; Stop closes
// done to hand control back to Go-Spring.
func (s *RestServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	rc := rest.RestConf{
		ServiceConf: service.ServiceConf{
			Name: s.cfg.Name,
			Log: logx.LogConf{
				ServiceName: s.cfg.Name,
				Level:       s.cfg.Log.Level,
			},
			Telemetry: trace.Config{
				Name:     s.cfg.Name,
				Endpoint: s.cfg.Tracing.Endpoint,
				Sampler:  s.cfg.Tracing.Sampler,
				Batcher:  s.cfg.Tracing.Batcher,
				Disabled: s.cfg.Tracing.Disabled,
			},
			DevServer: service.DevServerConfig{
				Enabled:       s.cfg.Metrics.Enabled,
				Port:          s.cfg.Metrics.Port,
				MetricsPath:   s.cfg.Metrics.Path,
				EnableMetrics: true,
				EnablePprof:   false,
			},
		},
		Host: s.cfg.Host,
		Port: s.cfg.Port,
	}
	s.svr = rest.MustNewServer(rc)
	s.reg(s.svr)

	// MustNewServer just ran ServiceConf.SetUp(), which called logx.SetUp() and
	// installed logx's own writer; replace it so go-zero's framework logs flow
	// into go-spring's log module. Level filtering above still applies.
	logx.SetWriter(logger.NewWriter())

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// Start binds the listener and blocks until Stop is called.
		s.svr.Start()
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-s.done:
		s.svr.Stop()
		return nil
	}
}

// Stop signals Run to return so Go-Spring can complete its shutdown sequence.
func (s *RestServer) Stop() error {
	close(s.done)
	return nil
}
