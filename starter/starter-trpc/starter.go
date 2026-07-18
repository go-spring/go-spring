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

package StarterTrpc

import (
	"context"
	"net"
	"strconv"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/server"
)

func init() {
	// Server side: gated on a ServiceRegister bean — no service to expose means
	// no server, so the client-only apps are never forced to stand one up.
	// Config is read from the ${spring.trpc.server} prefix.
	enableSimpleTrpcServer := gs.OnProperty("spring.trpc.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleTrpcServer, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(
			NewSimpleTrpcServer,
			gs.IndexArg(0, gs.TagArg("${spring.trpc.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister binds a service handler onto a tRPC server.Server. This
// function type keeps SimpleTrpcServer service-agnostic: it drives the
// lifecycle while each service supplies its own register bean, typically
// wrapping the generated xxx.RegisterXxxServiceService.
type ServiceRegister func(s *server.Server)

// Config defines the tRPC server configuration, bound from ${spring.trpc.server}.
//
// tRPC-Go's own trpc.NewServer() reads a trpc_go.yaml and owns configuration +
// plugin bootstrap globally. This starter takes the other fork: it builds a tRPC
// *Config programmatically from these Go-Spring properties and hands it to
// trpc.NewServerWithConfig, so there is no trpc_go.yaml and all config lives in
// conf/app.properties like every other Go-Spring service.
type Config struct {
	// Addr is the host:port the service listens on, split into tRPC's IP/Port.
	Addr string `value:"${addr:=127.0.0.1:8000}"`
	// ServiceName is the fully-qualified tRPC service name (trpc.app.server.service).
	// It must match the callee name baked into the generated client stub.
	ServiceName string `value:"${service.name:=trpc.helloworld.greet.GreetService}"`
	// Network / Protocol default to tRPC's own tcp + trpc wire protocol.
	Network  string `value:"${network:=tcp}"`
	Protocol string `value:"${protocol:=trpc}"`
}

// SimpleTrpcServer adapts a tRPC server.Server to the Go-Spring server
// lifecycle. tRPC's own trpc.NewServer()+Serve() blocks and owns the process;
// here the server implements gs.Server so Go-Spring drives startup and graceful
// shutdown alongside every other managed server.
//
// NOTE on signals: tRPC's server.Serve() installs its own OS signal handlers
// (SIGINT/SIGTERM/SIGSEGV/SIGUSR2). It co-exists with Go-Spring's lifecycle —
// Go-Spring's shutdown calls Stop(), which invokes server.Close(nil) to close
// the server's internal closeCh and unblock Serve. A direct SIGTERM is caught by
// both, which is harmless: whichever path fires first tears the server down.
type SimpleTrpcServer struct {
	cfg  Config
	reg  ServiceRegister
	svr  *server.Server
	done chan struct{}
}

// NewSimpleTrpcServer creates a SimpleTrpcServer from ${spring.trpc.server}
// config and the registered ServiceRegister bean.
func NewSimpleTrpcServer(cfg Config, reg ServiceRegister) *SimpleTrpcServer {
	return &SimpleTrpcServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// Run builds the tRPC server from a programmatic *trpc.Config (no trpc_go.yaml)
// and starts serving once Go-Spring signals readiness. server.Serve blocks
// internally, so it runs in a goroutine while Run parks on the done channel;
// Stop closes the server and done to hand control back to Go-Spring.
func (s *SimpleTrpcServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	host, portStr, err := net.SplitHostPort(s.cfg.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to parse addr %s", s.cfg.Addr)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return errutil.Explain(err, "failed to parse port from addr %s", s.cfg.Addr)
	}

	// Build the tRPC config in code instead of loading trpc_go.yaml, so the
	// whole service is configured through Go-Spring properties. NewServerWithConfig
	// repairs defaults and builds the service; plugin setup is intentionally left
	// out (this direct-connect starter uses no naming/registry plugins).
	cfg := &trpc.Config{}
	cfg.Server.Service = []*trpc.ServiceConfig{
		{
			Name:     s.cfg.ServiceName,
			IP:       host,
			Port:     uint16(port),
			Network:  s.cfg.Network,
			Protocol: s.cfg.Protocol,
		},
	}
	s.svr = trpc.NewServerWithConfig(cfg)

	// Bind the concrete service handler; the adapter itself stays service-agnostic.
	s.reg(s.svr)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// Serve binds the listener and blocks until Close/ a signal.
		errCh <- s.svr.Serve()
	}()

	select {
	case err = <-errCh:
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	case <-s.done:
		return nil
	}
}

// Stop closes the underlying tRPC server, which unblocks Serve, then signals Run
// to return so Go-Spring can complete shutdown.
func (s *SimpleTrpcServer) Stop() error {
	if s.svr != nil {
		_ = s.svr.Close(nil)
	}
	close(s.done)
	return nil
}
