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

package StarterGrpc

import (
	"context"
	"net"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"google.golang.org/grpc"
)

func init() {
	enableSimpleGrpcServer := gs.OnProperty("spring.grpc.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleGrpcServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register the gRPC server
		// when a service register is available.
		r.Provide(
			NewSimpleGrpcServer,
			gs.IndexArg(0, gs.TagArg("${spring.grpc.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister registers services on a grpc.Server.
type ServiceRegister func(svr *grpc.Server)

// Config defines gRPC server configuration.
type Config struct {
	Addr string `value:"${addr:=:9494}"`
}

// SimpleGrpcServer adapts a grpc.Server to the Go-Spring server lifecycle.
type SimpleGrpcServer struct {
	cfg Config
	reg ServiceRegister
	svr *grpc.Server
}

// NewSimpleGrpcServer creates a SimpleGrpcServer from ${spring.grpc.server} configuration.
func NewSimpleGrpcServer(cfg Config, reg ServiceRegister) *SimpleGrpcServer {
	return &SimpleGrpcServer{cfg: cfg, reg: reg}
}

// Run starts the gRPC server after Go-Spring signals readiness.
func (s *SimpleGrpcServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	s.svr = grpc.NewServer()
	s.reg(s.svr)

	listener, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to listen on %s", s.cfg.Addr)
	}
	<-sig.TriggerAndWait()
	if err = s.svr.Serve(listener); err != nil {
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	}
	return nil
}

// Stop gracefully stops the underlying gRPC server.
func (s *SimpleGrpcServer) Stop() error {
	s.svr.GracefulStop()
	return nil
}
