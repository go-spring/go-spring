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

package server

import (
	"context"
	"time"

	v1 "go-spring.org/go-kratos/api/helloworld/v1"
	"go-spring.org/go-kratos/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"go-spring.org/spring/gs"
)

// GRPCConfig is the gRPC server configuration, bound from ${spring.kratos.grpc}.
type GRPCConfig struct {
	Network string        `value:"${network:=}"`
	Addr    string        `value:"${addr:=0.0.0.0:9000}"`
	Timeout time.Duration `value:"${timeout:=1s}"`
}

// GRPCServer adapts a kratos gRPC transport server to the Go-Spring server
// lifecycle, mirroring HTTPServer.
type GRPCServer struct {
	svr *grpc.Server
}

// NewGRPCServer builds the kratos gRPC server from GRPCConfig and the injected
// GreeterService bean.
func NewGRPCServer(c GRPCConfig, greeter *service.GreeterService, logger log.Logger) *GRPCServer {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Network != "" {
		opts = append(opts, grpc.Network(c.Network))
	}
	if c.Addr != "" {
		opts = append(opts, grpc.Address(c.Addr))
	}
	if c.Timeout != 0 {
		opts = append(opts, grpc.Timeout(c.Timeout))
	}
	srv := grpc.NewServer(opts...)
	v1.RegisterGreeterServer(srv, greeter)
	return &GRPCServer{svr: srv}
}

// Run starts serving once Go-Spring signals readiness. kratos' Start binds the
// listener and blocks until Stop calls GracefulStop, at which point it returns.
func (s *GRPCServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	return s.svr.Start(ctx)
}

// Stop gracefully shuts down the kratos gRPC server.
func (s *GRPCServer) Stop() error {
	return s.svr.Stop(context.Background())
}
