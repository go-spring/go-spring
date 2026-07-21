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

// Package StarterGoFrameGRPC integrates goframe's grpcx.GrpcServer into the
// Go-Spring server lifecycle. Import it for the side effect and provide a
// ServiceRegister bean to attach a gRPC service:
//
//	import _ "go-spring.org/starter-goframe/grpc"
//
// Configuration is bound from the ${spring.goframe.grpc.server} prefix.
package StarterGoFrameGRPC

import (
	"context"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/net/gsvc"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
	"google.golang.org/grpc"

	// Side-effect import: installs the goframe (glog) -> go-spring log bridge
	// (see internal/logger). The bridge self-installs via init(), so importing
	// this package routes goframe's own logs into the application's go-spring
	// log pipeline before the first grpcx / gsvc call.
	_ "go-spring.org/starter-goframe/internal/logger"
)

func init() {
	// Importing the starter is the opt-in; the module still guards on
	// spring.goframe.grpc.server.enabled (default true). The grpcx server only
	// materialises when the application supplies a ServiceRegister bean, keeping
	// GRPCServer service-agnostic — each service registers its own handler.
	enabled := gs.OnProperty("spring.goframe.grpc.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enabled, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(NewGRPCServer, gs.IndexArg(0, gs.TagArg("${spring.goframe.grpc.server}"))).
			Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister binds a service handler onto the raw *grpc.Server that
// grpcx.GrpcServer wraps. This function type keeps GRPCServer service-agnostic:
// it drives the lifecycle while each service supplies its own register bean,
// typically wrapping the generated xxx.RegisterXxxServiceServer.
type ServiceRegister func(s grpc.ServiceRegistrar)

// Config binds goframe gRPC server settings from ${spring.goframe.grpc.server}.
//
// Observability note: tracing is deferred to starter-otel — grpcx instruments
// RPCs off the global OpenTelemetry TracerProvider that starter-otel installs.
type Config struct {
	Name    string `value:"${name:=goframe}"`
	Address string `value:"${address:=:8001}"`

	// Registry publishes the server into etcd for discovery. Leave etcd empty
	// (the default) for a plain server with no registration — grpcx reads
	// gsvc.GetRegistry() at construction, so an unset registry means no
	// Register/Deregister happens.
	Registry struct {
		Etcd string `value:"${etcd:=}"`
	} `value:"${registry}"`
}

// GRPCServer wraps a grpcx.GrpcServer so it satisfies gs.Server. The stock grpcx
// pattern is a blocking s.Run() in main(); here the server implements gs.Server
// so Go-Spring drives startup and graceful shutdown.
type GRPCServer struct {
	svr  *grpcx.GrpcServer
	done chan struct{}
}

// NewGRPCServer builds the grpcx server from the Go-Spring-bound config,
// optionally registers an etcd-backed gsvc.Registry globally *before*
// grpcx.Server.New is called (grpcx snapshots gsvc.GetRegistry() at construction
// time), and binds the service handler onto the underlying *grpc.Server via the
// injected ServiceRegister.
func NewGRPCServer(cfg Config, reg ServiceRegister) *GRPCServer {
	// Set the global registry first so grpcx.Server.New picks it up as its
	// registrar. Ordering matters: grpcx reads the registrar field at server
	// construction. Skipped when unconfigured.
	if cfg.Registry.Etcd != "" {
		gsvc.SetRegistry(etcdreg.New(cfg.Registry.Etcd))
	}

	grpcCfg := grpcx.Server.NewConfig()
	grpcCfg.Name = cfg.Name
	grpcCfg.Address = cfg.Address

	s := grpcx.Server.New(grpcCfg)
	reg(s.Server)

	return &GRPCServer{svr: s, done: make(chan struct{})}
}

// Run starts serving once Go-Spring signals readiness. grpcx.Server.Start is
// non-blocking (it binds the listener, spawns Serve in a goroutine and, when a
// registry is set, publishes the service into etcd), so Run blocks on `done`
// until Stop is called.
//
// Note: grpcx.Server.Run installs its own gproc signal handler, which would
// fight Go-Spring's signal handling. Using Start + park-on-done keeps shutdown
// owned by the Go-Spring lifecycle.
func (s *GRPCServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	s.svr.Start()
	<-s.done
	return nil
}

// Stop gracefully stops the underlying grpcx server (which deregisters from etcd
// when a registry is set and calls grpc.Server.GracefulStop) and unblocks Run.
func (s *GRPCServer) Stop() error {
	s.svr.Stop()
	close(s.done)
	return nil
}
