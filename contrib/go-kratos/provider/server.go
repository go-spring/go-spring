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
	"os"
	"time"

	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	kws "github.com/tx7do/kratos-transport/transport/websocket"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// The kratos logger is shared by the server adapter (and would be shared by
	// any service that needs it). The scaffold built it inline in main() and
	// passed it down every layer; here it is a plain Go-Spring bean.
	gs.Provide(NewLogger)

	// Register the kratos server adapter and bind it to the Go-Spring server
	// lifecycle. Config is filled from the ${spring.kratos} prefix. The server
	// only materializes when a ServiceRegister bean exists, mirroring how the
	// dubbo-go sample gates on ServiceRegister rather than any concrete service.
	gs.Provide(NewKratosServer, gs.IndexArg(0, gs.TagArg("${spring.kratos}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[ServiceRegister]())
}

// NewLogger provides the kratos logger used by the server adapter.
func NewLogger() log.Logger {
	return log.NewStdLogger(os.Stdout)
}

// ServiceRegister binds services onto the kratos HTTP, gRPC and WebSocket
// transport servers. Extracting the registration behind this function type
// keeps KratosServer service-agnostic: it drives the lifecycle while each
// service supplies its own register bean.
//
// WebSocket is added alongside HTTP+gRPC (not in a separate sub-project)
// because kratos-transport's websocket.Server implements the same
// transport.Server interface, so it CAN coexist inside a single kratos.App —
// they share the App name, the etcd Registrar, and the graceful-shutdown flow.
// A separate subdir would be warranted only when a transport genuinely cannot
// coexist (e.g. it needs an external broker like MQTT's mosquitto).
type ServiceRegister func(hs *khttp.Server, gs *kgrpc.Server, ws *kws.Server) error

// Config defines kratos server + etcd registry configuration, bound from
// ${spring.kratos}.
type Config struct {
	Name         string        `value:"${name:=kratos-greeter}"`
	HTTPNetwork  string        `value:"${http.network:=}"`
	HTTPAddr     string        `value:"${http.addr:=0.0.0.0:8000}"`
	HTTPTimeout  time.Duration `value:"${http.timeout:=1s}"`
	GRPCNetwork  string        `value:"${grpc.network:=}"`
	GRPCAddr     string        `value:"${grpc.addr:=0.0.0.0:9000}"`
	GRPCTimeout  time.Duration `value:"${grpc.timeout:=1s}"`
	WSNetwork    string        `value:"${ws.network:=}"`
	WSAddr       string        `value:"${ws.addr:=0.0.0.0:9002}"`
	WSPath       string        `value:"${ws.path:=/}"`
	RegistryAddr string        `value:"${registry.etcd:=127.0.0.1:2379}"`
}

// KratosServer adapts a *kratos.App to the Go-Spring server lifecycle. This is
// the whole point of the refactor: the scaffold wrapped each transport server
// as its own gs.Server bean and had no registry, which fragmented service
// registration (kratos publishes an App, not a single transport). Here the
// HTTP, gRPC and WebSocket transport servers are composed into one kratos.App
// together with an etcd Registrar, and the App implements gs.Server so
// Go-Spring drives startup and graceful shutdown alongside every other managed
// server. The three transports share a single kratos.App name and are all
// published into etcd under that name, tagged by kratos "kind" ("http",
// "grpc", "websocket").
type KratosServer struct {
	cfg  Config
	reg  ServiceRegister
	log  log.Logger
	app  *kratos.App
	done chan struct{}
}

// NewKratosServer creates a KratosServer from ${spring.kratos} config, the
// registered ServiceRegister bean, and the shared kratos logger.
func NewKratosServer(cfg Config, reg ServiceRegister, logger log.Logger) *KratosServer {
	return &KratosServer{cfg: cfg, reg: reg, log: logger, done: make(chan struct{})}
}

// Run builds the kratos HTTP, gRPC and WebSocket transport servers, composes
// them into a kratos.App together with the etcd Registrar, and starts serving
// once Go-Spring signals readiness. kratos.App.Run publishes the service
// instance into etcd (under the configured name) and blocks until Stop is
// called, so it runs in a goroutine while Run parks on the done channel; Stop
// closes done to hand control back to Go-Spring after tearing the App down.
func (s *KratosServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	httpOpts := []khttp.ServerOption{
		khttp.Middleware(recovery.Recovery()),
	}
	if s.cfg.HTTPNetwork != "" {
		httpOpts = append(httpOpts, khttp.Network(s.cfg.HTTPNetwork))
	}
	if s.cfg.HTTPAddr != "" {
		httpOpts = append(httpOpts, khttp.Address(s.cfg.HTTPAddr))
	}
	if s.cfg.HTTPTimeout != 0 {
		httpOpts = append(httpOpts, khttp.Timeout(s.cfg.HTTPTimeout))
	}
	httpSrv := khttp.NewServer(httpOpts...)

	grpcOpts := []kgrpc.ServerOption{
		kgrpc.Middleware(recovery.Recovery()),
	}
	if s.cfg.GRPCNetwork != "" {
		grpcOpts = append(grpcOpts, kgrpc.Network(s.cfg.GRPCNetwork))
	}
	if s.cfg.GRPCAddr != "" {
		grpcOpts = append(grpcOpts, kgrpc.Address(s.cfg.GRPCAddr))
	}
	if s.cfg.GRPCTimeout != 0 {
		grpcOpts = append(grpcOpts, kgrpc.Timeout(s.cfg.GRPCTimeout))
	}
	grpcSrv := kgrpc.NewServer(grpcOpts...)

	// WebSocket transport server. Unlike HTTP+gRPC (whose contracts come from
	// helloworld.proto), WebSocket carries application-defined framed messages.
	// We use PayloadTypeBinary because kratos-transport's text-mode server has
	// an asymmetric quirk in the pinned version: it unwraps a
	// `{"type","payload"}` envelope on receive but sends back just the raw
	// codec bytes with no envelope, which forces the client to speak two
	// different formats depending on direction. Binary mode is symmetric:
	// every frame on the wire is
	//   <4-byte little-endian uint32 messageType><JSON-encoded payload bytes>
	// so the consumer can hand-craft one format and expect the same shape back.
	//
	// The kratos-transport WS dep is pinned to v1.3.1 in go.mod: v1.3.4
	// introduced a regression where the wsHandler no longer registers the
	// session with the SessionManager, so Server.SendMessage always fails with
	// "session not found" and no reply ever reaches the client. v1.3.1's
	// register-channel-based session handoff still works correctly.
	wsOpts := []kws.ServerOption{
		kws.WithPath(s.cfg.WSPath),
		kws.WithCodec("json"),
		kws.WithPayloadType(kws.PayloadTypeBinary),
	}
	if s.cfg.WSNetwork != "" {
		wsOpts = append(wsOpts, kws.WithNetwork(s.cfg.WSNetwork))
	}
	if s.cfg.WSAddr != "" {
		wsOpts = append(wsOpts, kws.WithAddress(s.cfg.WSAddr))
	}
	wsSrv := kws.NewServer(wsOpts...)

	if err := s.reg(httpSrv, grpcSrv, wsSrv); err != nil {
		return errutil.Explain(err, "failed to register kratos service")
	}

	// Registry turns the direct-connect example into a real service: the App
	// publishes {Name, endpoints} into etcd on start and Deregisters on stop.
	// All three transports register under the same App name; consumers can
	// filter by kratos "kind" if they need to pick a specific transport.
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{s.cfg.RegistryAddr},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return errutil.Explain(err, "failed to create etcd client for %s", s.cfg.RegistryAddr)
	}
	r := etcd.New(cli)

	s.app = kratos.New(
		kratos.Name(s.cfg.Name),
		kratos.Logger(s.log),
		kratos.Server(httpSrv, grpcSrv, wsSrv),
		kratos.Registrar(r),
	)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// App.Run starts every transport server and blocks; on Stop it
		// deregisters from etcd and shuts down each transport in turn.
		errCh <- s.app.Run()
	}()

	select {
	case err = <-errCh:
		return errutil.Explain(err, "kratos app exited with error")
	case <-s.done:
		return s.app.Stop()
	}
}

// Stop signals Run to tear down the kratos.App so Go-Spring can complete its
// shutdown sequence.
func (s *KratosServer) Stop() error {
	close(s.done)
	return nil
}
