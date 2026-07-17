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

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/trace"
	"github.com/zeromicro/go-zero/zrpc"
	"go-spring.org/spring/gs"
	"google.golang.org/grpc"
)

func init() {
	// Register the zrpc server and bind it to the Go-Spring server lifecycle.
	// Config is filled from the ${spring.zrpc.server} prefix. The server only
	// materializes when a ServiceRegister bean exists, mirroring how the
	// dubbo-go example gates on ServiceRegister rather than any concrete
	// service — keeping ZrpcServer service-agnostic.
	gs.Provide(NewZrpcServer, gs.IndexArg(0, gs.TagArg("${spring.zrpc.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[ServiceRegister]())
}

// ServiceRegister registers services on a grpc.Server. Extracting the
// registration behind this function type keeps ZrpcServer independent of any
// concrete pb-generated interface; each service supplies its own register bean.
type ServiceRegister func(grpcServer *grpc.Server)

// Config defines zrpc server configuration, bound from ${spring.zrpc.server}.
//
// go-zero's REST framework (rest.Server) has no built-in service discovery,
// so the whole registry story only exists in zrpc. That is the crucial
// difference from the other contrib examples: to demonstrate real go-zero
// service governance we must run a zrpc (gRPC) server, not a rest.Server.
//
// zrpc.RpcServerConf embeds service.ServiceConf (just like rest.RestConf
// does), so this example surfaces the same three observability pillars go-zero
// wires up natively inside zrpc.MustNewServer → ServiceConf.SetUp(): tracing
// (Telemetry), metrics (DevServer /metrics) and logging (logx). We do NOT
// hand-wire any OpenTelemetry/Prometheus code — the zrpc server's default
// unary/stream interceptors (trace/prometheus/stat) do the instrumentation
// once these config fields are populated.
//
// Because we build the RpcServerConf struct in Go rather than loading it
// through go-zero's conf loader, go-zero's `json:",default="` tags do NOT
// apply; every field must carry an explicit value, so each maps to a value-tag
// default that keeps the plain smoke test working even without the
// observability backends.
type Config struct {
	Name     string `value:"${name:=greet-rpc}"`
	ListenOn string `value:"${listen-on:=0.0.0.0:8081}"`
	EtcdAddr string `value:"${etcd.addr:=127.0.0.1:2379}"`
	EtcdKey  string `value:"${etcd.key:=greet.rpc}"`

	// Tracing → OTel → Jaeger over OTLP/gRPC (docker-compose.yml). Disabled
	// defaults to false so a locally-running backend is used when present; the
	// smoke test still starts fine when nothing listens on the endpoint (the
	// exporter just fails to connect in the background).
	TracingEndpoint string  `value:"${tracing.endpoint:=127.0.0.1:4317}"`
	TracingSampler  float64 `value:"${tracing.sampler:=1.0}"`
	TracingBatcher  string  `value:"${tracing.batcher:=otlpgrpc}"`
	TracingDisabled bool    `value:"${tracing.disabled:=false}"`

	// Metrics: go-zero's DevServer exposes a Prometheus scrape endpoint
	// (/metrics) plus health on its own port, independent of the zrpc port.
	// The zrpc prometheus interceptor records rpc_server_requests_* into the
	// default registry that this endpoint serves.
	MetricsEnabled bool   `value:"${metrics.enabled:=true}"`
	MetricsPort    int    `value:"${metrics.port:=6060}"`
	MetricsPath    string `value:"${metrics.path:=/metrics}"`

	// Logging: go-zero's logx writes structured JSON. In file mode it emits
	// access/error/stat/severe/slow .log files under LogPath, each line
	// carrying trace/span keys so logs correlate with Jaeger spans out of the
	// box.
	LogMode     string `value:"${log.mode:=file}"`
	LogEncoding string `value:"${log.encoding:=json}"`
	LogPath     string `value:"${log.path:=../logs}"`
	LogLevel    string `value:"${log.level:=info}"`
}

// ZrpcServer adapts a zrpc.RpcServer to the Go-Spring server lifecycle. The
// stock go-zero pattern is `srv.Start()` blocking in main(); here the server
// instead implements gs.Server so Go-Spring drives startup and graceful
// shutdown alongside every other managed server.
type ZrpcServer struct {
	cfg  Config
	reg  ServiceRegister
	svr  *zrpc.RpcServer
	done chan struct{}
}

// NewZrpcServer builds a ZrpcServer from ${spring.zrpc.server} config and the
// registered ServiceRegister bean.
func NewZrpcServer(cfg Config, reg ServiceRegister) *ZrpcServer {
	return &ZrpcServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// Run builds the zrpc server on the configured ListenOn address, wires in the
// etcd registration (Etcd.Hosts + Etcd.Key) so the provider publishes itself
// under the given key on Start, and then serves once Go-Spring signals
// readiness. zrpc's Start blocks internally, so it runs in a goroutine while
// Run parks on the done channel; Stop closes done to hand control back to
// Go-Spring.
func (s *ZrpcServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	conf := zrpc.RpcServerConf{
		ServiceConf: service.ServiceConf{
			Name: s.cfg.Name,
			// logx: level is still applied by logx before it delegates to the
			// Writer we install after MustNewServer (see logbridge.go), so
			// LogConf.Level is the only field that still matters at runtime;
			// Mode / Encoding / Path become no-ops once SetWriter replaces
			// logx's own file writer. Kept here to document what the ambient
			// go-zero config would have produced.
			Log: logx.LogConf{
				ServiceName: s.cfg.Name,
				Mode:        s.cfg.LogMode,
				Encoding:    s.cfg.LogEncoding,
				Path:        s.cfg.LogPath,
				Level:       s.cfg.LogLevel,
			},
			// Telemetry: OTel tracing exported to Jaeger over OTLP/gRPC. The
			// zrpc trace interceptor starts a span per RPC.
			Telemetry: trace.Config{
				Name:     s.cfg.Name,
				Endpoint: s.cfg.TracingEndpoint,
				Sampler:  s.cfg.TracingSampler,
				Batcher:  s.cfg.TracingBatcher,
				Disabled: s.cfg.TracingDisabled,
			},
			// DevServer: serves the Prometheus /metrics endpoint (pprof off to
			// keep it lean). The zrpc prometheus interceptor feeds the default
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
		ListenOn: s.cfg.ListenOn,
		// Etcd turns the direct-connect example into a real service: on Start
		// the provider registers its ListenOn address under cfg.EtcdKey in
		// etcd, and the consumer resolves that same key to find it.
		Etcd: discov.EtcdConf{
			Hosts: []string{s.cfg.EtcdAddr},
			Key:   s.cfg.EtcdKey,
		},
	}
	s.svr = zrpc.MustNewServer(conf, func(grpcServer *grpc.Server) {
		s.reg(grpcServer)
	})

	// Redirect logx into go-spring's log module. MustNewServer just ran
	// ServiceConf.SetUp(), which called logx.SetUp() with the LogConf above and
	// installed logx's own file writer; SetWriter now replaces that writer so
	// every subsequent framework log line (interceptor errors, etcd registrar,
	// stat lines, ...) flows through logbridge.go into the root FileLogger
	// alongside the business logs. LogConf.Level is still honoured because
	// logx filters by level before delegating to the Writer; Mode / Encoding /
	// Path become moot for this process. See provider/logbridge.go.
	logx.SetWriter(newGSBridgeWriter())

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// Start binds the listener, registers the provider under EtcdKey in
		// etcd, and blocks.
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
func (s *ZrpcServer) Stop() error {
	close(s.done)
	return nil
}
