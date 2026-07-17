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

// Package StarterGoZeroZrpc integrates go-zero's zrpc.RpcServer (gRPC, with
// built-in etcd service discovery) into the Go-Spring server lifecycle. Import
// it for the side effect and provide a ServiceRegister bean to bind services:
//
//	import _ "go-spring.org/starter-go-zero/zrpc"
//
// Configuration is bound from the ${spring.go-zero.zrpc.server} prefix. Unlike
// rest.Server, zrpc carries the whole service-governance story — leave Etcd
// empty for a plain direct-connect server, or set it to publish into etcd.
package StarterGoZeroZrpc

import (
	"context"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/trace"
	"github.com/zeromicro/go-zero/zrpc"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
	"google.golang.org/grpc"

	"go-spring.org/starter-go-zero/internal/logbridge"
)

func init() {
	// Importing the starter is the opt-in; the module still guards on
	// spring.go-zero.zrpc.server.enabled (default true). The zrpc server only
	// materializes when the application supplies a ServiceRegister bean, keeping
	// ZrpcServer independent of any concrete pb-generated service.
	enabled := gs.OnProperty("spring.go-zero.zrpc.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enabled, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(NewZrpcServer, gs.IndexArg(0, gs.TagArg("${spring.go-zero.zrpc.server}"))).
			Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister registers services on a grpc.Server. Extracting registration
// behind this function type keeps ZrpcServer independent of any concrete
// pb-generated interface; each service supplies its own register bean.
type ServiceRegister func(grpcServer *grpc.Server)

// Config binds go-zero zrpc.RpcServer configuration from
// ${spring.go-zero.zrpc.server}.
//
// Etcd is optional: leave Etcd.Addr empty for a plain direct-connect server, or
// set Addr+Key to publish the ListenOn address into etcd on startup so consumers
// can resolve it. Observability follows the same policy as the rest starter:
// tracing is deferred to starter-otel (Tracing.Disabled defaults to true so the
// zrpc trace interceptor uses the ambient global OTel provider); metrics stay
// go-zero-native via the DevServer, off by default.
type Config struct {
	Name     string `value:"${name:=go-zero}"`
	ListenOn string `value:"${listen-on:=0.0.0.0:8081}"`

	// Etcd service discovery. Empty Addr disables registration (direct-connect).
	Etcd struct {
		Addr string `value:"${addr:=}"`
		Key  string `value:"${key:=}"`
	} `value:"${etcd}"`

	// Tracing: defer to starter-otel by default (Disabled=true). Endpoint /
	// Sampler / Batcher only take effect when Disabled=false.
	Tracing struct {
		Disabled bool    `value:"${disabled:=true}"`
		Endpoint string  `value:"${endpoint:=}"`
		Sampler  float64 `value:"${sampler:=1.0}"`
		Batcher  string  `value:"${batcher:=otlpgrpc}"`
	} `value:"${tracing}"`

	// Metrics: go-zero's DevServer exposes a Prometheus /metrics endpoint on its
	// own port. Off by default.
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

// ZrpcServer adapts a zrpc.RpcServer to the Go-Spring server lifecycle. The
// stock go-zero pattern is a blocking srv.Start() in main(); here the server
// implements gs.Server so Go-Spring drives startup and graceful shutdown
// alongside every other managed server.
type ZrpcServer struct {
	cfg  Config
	reg  ServiceRegister
	svr  *zrpc.RpcServer
	done chan struct{}
}

// NewZrpcServer builds a ZrpcServer from ${spring.go-zero.zrpc.server} config
// and the registered ServiceRegister bean.
func NewZrpcServer(cfg Config, reg ServiceRegister) *ZrpcServer {
	return &ZrpcServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// Run builds the zrpc server on the configured ListenOn address, optionally
// wiring etcd registration, then serves once Go-Spring signals readiness. zrpc's
// Start blocks internally, so it runs in a goroutine while Run parks on the done
// channel; Stop closes done to hand control back to Go-Spring.
func (s *ZrpcServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	conf := zrpc.RpcServerConf{
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
		ListenOn: s.cfg.ListenOn,
	}
	// Only publish into etcd when an address is configured; otherwise the server
	// is a plain direct-connect gRPC endpoint.
	if s.cfg.Etcd.Addr != "" {
		conf.Etcd = discov.EtcdConf{
			Hosts: []string{s.cfg.Etcd.Addr},
			Key:   s.cfg.Etcd.Key,
		}
	}
	s.svr = zrpc.MustNewServer(conf, func(grpcServer *grpc.Server) {
		s.reg(grpcServer)
	})

	// MustNewServer just ran ServiceConf.SetUp(), which called logx.SetUp() and
	// installed logx's own writer; replace it so go-zero's framework logs flow
	// into go-spring's log module. Level filtering above still applies.
	logx.SetWriter(logbridge.NewWriter())

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// Start binds the listener, registers the provider under Etcd.Key when
		// configured, and blocks.
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
