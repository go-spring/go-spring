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
// grpcx.GrpcServer to the Go-Spring server lifecycle. In a stock grpcx
// scaffold this wiring lives in main() as
// `s := grpcx.Server.New(cfg); echo.RegisterEchoServiceServer(s.Server, impl); s.Run()`.
// Here it is expressed as a gs.Server bean so the container drives startup
// and graceful shutdown, and it wires goframe's built-in etcd registry so the
// provider publishes itself into etcd instead of being reached via a
// hard-coded host:port.
package main

import (
	"context"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/net/gsvc"
	"go-spring.org/spring/gs"
	"google.golang.org/grpc"
)

func init() {
	// Register the grpcx server and bind it to the Go-Spring server lifecycle.
	// Config is filled from the ${goframe.grpc} prefix. The server only
	// materialises when a ServiceRegister bean exists, mirroring how the kitex
	// example gates its server on the injected register bean rather than any
	// concrete generated service.
	gs.Provide(NewGoFrameGrpcServer, gs.IndexArg(0, gs.TagArg("${goframe.grpc}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[ServiceRegister]())
}

// ServiceRegister binds a service handler onto the raw *grpc.Server that
// grpcx.GrpcServer wraps. This function type keeps GoFrameGrpcServer
// service-agnostic: it drives the lifecycle while each service supplies its own
// register bean, typically wrapping the generated xxx.RegisterXxxServiceServer.
type ServiceRegister func(s grpc.ServiceRegistrar)

// Config holds the goframe gRPC server settings. In a stock goframe project
// grpcx.Server.NewConfig() reads these from manifest/config/config.yaml under
// the "grpc:" node via g.Cfg(); here they are bound from Go-Spring properties
// (see conf/app.properties) under the "${goframe.grpc}" prefix using `value`
// tags, so the config source moves out of goframe's own loader.
type Config struct {
	Address      string `value:"${address:=:8001}"`
	Name         string `value:"${name:=goframe.grpc.echo}"`
	RegistryAddr string `value:"${registry.etcd:=127.0.0.1:2379}"`
}

// GoFrameGrpcServer wraps a grpcx.GrpcServer so it satisfies gs.Server.
type GoFrameGrpcServer struct {
	cfg  Config
	reg  ServiceRegister
	svr  *grpcx.GrpcServer
	done chan struct{}
}

// NewGoFrameGrpcServer builds the grpcx server from the Go-Spring-bound
// config, registers an etcd-backed gsvc.Registry globally *before*
// grpcx.Server.New is called (grpcx.GrpcServer snapshots gsvc.GetRegistry() at
// construction time — see grpcx_grpc_server.go: `registrar: gsvc.GetRegistry()`),
// and binds the service handler onto the underlying *grpc.Server via the
// injected ServiceRegister. When Start is invoked later, grpcx will publish the
// service under cfg.Name into etcd; on Stop it deregisters itself.
func NewGoFrameGrpcServer(cfg Config, reg ServiceRegister) *GoFrameGrpcServer {
	// Route goframe's own glog logs into go-spring's log module (see
	// logbridge.go). Installed before any grpcx / gsvc call so even the very
	// first framework log lands in the same pipeline as the business logs.
	installGoFrameLogBridge()

	// Set the global registry first so grpcx.Server.New picks it up as its
	// registrar. Ordering matters: see grpcx_grpc_server.go, where the
	// registrar field is read at server construction.
	gsvc.SetRegistry(etcdreg.New(cfg.RegistryAddr))

	grpcCfg := grpcx.Server.NewConfig()
	grpcCfg.Name = cfg.Name
	grpcCfg.Address = cfg.Address

	s := grpcx.Server.New(grpcCfg)
	reg(s.Server)

	return &GoFrameGrpcServer{
		cfg:  cfg,
		reg:  reg,
		svr:  s,
		done: make(chan struct{}),
	}
}

// Run starts serving once Go-Spring signals readiness. grpcx.Server.Start is
// non-blocking (it binds the listener, spawns Serve in a goroutine and
// publishes the service into etcd), so Run blocks on `done` until Stop is
// called, keeping the server bean alive for the container.
//
// Note: grpcx.Server.Run installs its own gproc signal handler, which would
// fight Go-Spring's signal handling. Using Start + park-on-done keeps
// shutdown owned by the Go-Spring lifecycle.
func (s *GoFrameGrpcServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	s.svr.Start()
	<-s.done
	return nil
}

// Stop gracefully stops the underlying grpcx server (which deregisters from
// etcd and calls grpc.Server.GracefulStop) and unblocks Run. This replaces the
// process-owned gproc signal handling that grpcx.Server.Run would otherwise
// install.
func (s *GoFrameGrpcServer) Stop() error {
	s.svr.Stop()
	close(s.done)
	return nil
}
