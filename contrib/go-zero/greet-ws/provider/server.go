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

package main

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/trace"
	"github.com/zeromicro/go-zero/rest"
	"go-spring.org/spring/gs"
)

func init() {
	// Register the go-zero REST server and bind it to the Go-Spring server
	// lifecycle. Config is filled from the ${spring.rest.server} prefix. The
	// server only materializes when a HandlerRegister bean exists, keeping
	// RestServer service-agnostic — same shape as the sibling greet-api.
	//
	// Why the same rest.Server hosts a WebSocket route: go-zero has no
	// dedicated WS server type. WS is served as a hand-written route on
	// rest.Server whose handler upgrades the connection. That means the
	// adapter code below is deliberately identical to greet-api's — only
	// what the HandlerRegister does inside changes.
	gs.Provide(NewRestServer, gs.IndexArg(0, gs.TagArg("${spring.rest.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[HandlerRegister]())
}

// HandlerRegister registers handlers onto a *rest.Server. Extracting the
// registration behind this function type keeps RestServer independent of any
// concrete route table; each service supplies its own register bean.
type HandlerRegister func(server *rest.Server)

// Config defines go-zero REST server configuration, bound from
// ${spring.rest.server}.
//
// WebSocket rides on the same rest.RestConf as greet-api — there are no
// extra WS-specific knobs — so the observability shape is identical: Name /
// Host / Port plus the three pillars go-zero wires up natively inside
// rest.MustNewServer → ServiceConf.SetUp(): tracing (Telemetry), metrics
// (DevServer /metrics) and logging (logx). We do NOT hand-wire any
// OpenTelemetry/Prometheus code — the rest.Server middlewares
// (Trace/Prometheus/Metrics/Log, all on by default) do the instrumentation
// once these config fields are populated.
//
// Because we build the RestConf struct in Go rather than loading it through
// go-zero's conf loader, go-zero's `json:",default="` tags do NOT apply; every
// field must carry an explicit value, so each maps to a value-tag default that
// keeps the plain smoke test working even without the observability backends.
type Config struct {
	Name string `value:"${name:=greet-ws}"`
	Host string `value:"${host:=0.0.0.0}"`
	Port int    `value:"${port:=8890}"`

	// Tracing → OTel → Jaeger over OTLP/gRPC (docker-compose.yml). Disabled
	// defaults to false so a locally-running backend is used when present; the
	// smoke test still starts fine when nothing listens on the endpoint (the
	// exporter just fails to connect in the background).
	TracingEndpoint string  `value:"${tracing.endpoint:=127.0.0.1:4317}"`
	TracingSampler  float64 `value:"${tracing.sampler:=1.0}"`
	TracingBatcher  string  `value:"${tracing.batcher:=otlpgrpc}"`
	TracingDisabled bool    `value:"${tracing.disabled:=false}"`

	// Metrics: go-zero's DevServer exposes a Prometheus scrape endpoint
	// (/metrics) plus health on its own port, independent of the REST port.
	// The Prometheus middleware records http_server_requests_* into the default
	// registry that this endpoint serves. Note: because WS is a long-lived
	// connection, http_server_requests_* is only recorded when the connection
	// closes; go_* process metrics from the same registry are always live.
	MetricsEnabled bool   `value:"${metrics.enabled:=true}"`
	MetricsPort    int    `value:"${metrics.port:=6060}"`
	MetricsPath    string `value:"${metrics.path:=/metrics}"`

	// Logging: go-zero's logx writes structured JSON. In file mode it emits
	// access/error/stat/severe/slow .log files under LogPath, each line carrying
	// trace/span keys so logs correlate with Jaeger spans out of the box.
	LogMode     string `value:"${log.mode:=file}"`
	LogEncoding string `value:"${log.encoding:=json}"`
	LogPath     string `value:"${log.path:=../logs}"`
	LogLevel    string `value:"${log.level:=info}"`
}

// RestServer adapts a go-zero rest.Server to the Go-Spring server lifecycle.
// Identical pattern to the sibling greet-api: rest.Server.Start blocks
// internally, so it runs in a goroutine while Run parks on a done channel,
// and Stop closes done to trigger rest.Server.Stop.
type RestServer struct {
	cfg  Config
	reg  HandlerRegister
	svr  *rest.Server
	done chan struct{}
}

// NewRestServer builds a RestServer from ${spring.rest.server} config and
// the registered HandlerRegister bean.
func NewRestServer(cfg Config, reg HandlerRegister) *RestServer {
	return &RestServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// Run builds the rest.Server on the configured Host:Port, hands it to the
// HandlerRegister bean to attach routes (including the WS upgrade route),
// and then serves once Go-Spring signals readiness.
func (s *RestServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	rc := rest.RestConf{
		ServiceConf: service.ServiceConf{
			Name: s.cfg.Name,
			// logx: level is still applied by logx before it delegates to the
			// Writer we install after MustNewServer (see logbridge.go), so
			// LogConf.Level is the only field that still matters at runtime;
			// Mode / Encoding / Path become no-ops once SetWriter replaces
			// logx's own file writer.
			Log: logx.LogConf{
				ServiceName: s.cfg.Name,
				Mode:        s.cfg.LogMode,
				Encoding:    s.cfg.LogEncoding,
				Path:        s.cfg.LogPath,
				Level:       s.cfg.LogLevel,
			},
			// Telemetry: OTel tracing exported to Jaeger over OTLP/gRPC. The
			// rest.Server trace middleware starts a span per request (including
			// the WS upgrade request).
			Telemetry: trace.Config{
				Name:     s.cfg.Name,
				Endpoint: s.cfg.TracingEndpoint,
				Sampler:  s.cfg.TracingSampler,
				Batcher:  s.cfg.TracingBatcher,
				Disabled: s.cfg.TracingDisabled,
			},
			// DevServer: serves the Prometheus /metrics endpoint (pprof off to
			// keep it lean). The Prometheus middleware feeds the default
			// registry this endpoint exposes.
			DevServer: service.DevServerConfig{
				Enabled:        s.cfg.MetricsEnabled,
				Port:           s.cfg.MetricsPort,
				MetricsPath:    s.cfg.MetricsPath,
				EnableMetrics:  true,
				EnablePprof:    false,
				HealthResponse: "OK",
			},
		},
		Host: s.cfg.Host,
		Port: s.cfg.Port,
	}
	s.svr = rest.MustNewServer(rc)
	s.reg(s.svr)

	// Redirect logx into go-spring's log module. MustNewServer just ran
	// ServiceConf.SetUp(), which called logx.SetUp() with the LogConf above and
	// installed logx's own file writer; SetWriter now replaces that writer so
	// every subsequent framework log line (access log, ws upgrade errors,
	// stat lines, ...) flows through logbridge.go into the root FileLogger
	// alongside the business logs. LogConf.Level is still honoured because
	// logx filters by level before delegating to the Writer; Mode / Encoding /
	// Path become moot for this process. See provider/logbridge.go.
	logx.SetWriter(newGSBridgeWriter())

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
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
