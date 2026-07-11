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

	"dubbo.apache.org/dubbo-go/v3/protocol"
	"dubbo.apache.org/dubbo-go/v3/server"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register the Dubbo server and bind it to the Go-Spring server lifecycle.
	// Config is filled from the ${spring.dubbo.server} prefix. The server only
	// materializes when a ServiceRegister bean exists, mirroring how the
	// grpc starter gates on ServiceRegister rather than any concrete service.
	gs.Provide(NewDubboServer, gs.IndexArg(0, gs.TagArg("${spring.dubbo.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[ServiceRegister]())
}

// ServiceRegister registers services on a Dubbo server.Server. Extracting the
// registration behind this function type keeps DubboServer service-agnostic:
// it drives the lifecycle while each service supplies its own register bean.
type ServiceRegister func(svr *server.Server) error

// Config defines Dubbo server configuration, bound from ${spring.dubbo.server}.
type Config struct {
	Port int `value:"${port:=20000}"`
}

// DubboServer adapts a Dubbo-go server.Server to the Go-Spring server lifecycle.
// This is the whole point of the refactor: the scaffold called srv.Serve()
// directly from main(), which blocks and owns the process. Here the server
// instead implements gs.Server so Go-Spring drives startup and graceful
// shutdown alongside every other managed server.
type DubboServer struct {
	cfg  Config
	reg  ServiceRegister
	svr  *server.Server
	done chan struct{}
}

// NewDubboServer creates a DubboServer from ${spring.dubbo.server} config and
// the registered ServiceRegister bean.
func NewDubboServer(cfg Config, reg ServiceRegister) *DubboServer {
	return &DubboServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// Run builds the Dubbo triple server on the configured port and starts serving
// once Go-Spring signals readiness. Dubbo's Serve blocks forever internally, so
// it runs in a goroutine while Run parks on the done channel; Stop closes done
// to hand control back to Go-Spring's shutdown sequence.
func (s *DubboServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	svr, err := server.NewServer(
		server.WithServerProtocol(
			protocol.WithPort(s.cfg.Port),
			protocol.WithTriple(),
		),
	)
	if err != nil {
		return errutil.Explain(err, "failed to create dubbo server")
	}
	if err = s.reg(svr); err != nil {
		return errutil.Explain(err, "failed to register dubbo service")
	}
	s.svr = svr

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// Serve exports the service (binding the listener) and then blocks.
		errCh <- svr.Serve()
	}()

	select {
	case err = <-errCh:
		return errutil.Explain(err, "failed to serve on port %d", s.cfg.Port)
	case <-s.done:
		return nil
	}
}

// Stop signals Run to return so Go-Spring can complete its shutdown sequence.
func (s *DubboServer) Stop() error {
	close(s.done)
	return nil
}
