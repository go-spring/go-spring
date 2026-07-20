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
	"time"

	"go-spring.org/spring/cloud/tlsconf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
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

// KeepaliveConfig tunes server-side keepalive enforcement. Zero values leave
// the corresponding gRPC default in place.
type KeepaliveConfig struct {
	Time              time.Duration `value:"${time:=0}"`
	Timeout           time.Duration `value:"${timeout:=0}"`
	MaxConnectionIdle time.Duration `value:"${maxConnectionIdle:=0}"`
	MaxConnectionAge  time.Duration `value:"${maxConnectionAge:=0}"`
}

// HealthConfig toggles the standard grpc_health_v1 health service. It is
// enabled by default because it is the conventional way to expose gRPC
// readiness to load balancers and probes.
type HealthConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// Config defines gRPC server configuration.
type Config struct {
	Addr                 string             `value:"${addr:=:9494}"`
	ConnectionTimeout    time.Duration      `value:"${connectionTimeout:=0}"`
	MaxRecvMsgSize       int                `value:"${maxRecvMsgSize:=0}"`
	MaxSendMsgSize       int                `value:"${maxSendMsgSize:=0}"`
	MaxConcurrentStreams uint32             `value:"${maxConcurrentStreams:=0}"`
	Keepalive            KeepaliveConfig    `value:"${keepalive}"`
	TLS                  tlsconf.TLSConfig `value:"${tls}"`
	Health               HealthConfig       `value:"${health}"`
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

// buildOptions translates the bound Config into grpc.ServerOption values.
func (s *SimpleGrpcServer) buildOptions() ([]grpc.ServerOption, error) {
	var opts []grpc.ServerOption
	if s.cfg.ConnectionTimeout > 0 {
		opts = append(opts, grpc.ConnectionTimeout(s.cfg.ConnectionTimeout))
	}
	if s.cfg.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(s.cfg.MaxRecvMsgSize))
	}
	if s.cfg.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(s.cfg.MaxSendMsgSize))
	}
	if s.cfg.MaxConcurrentStreams > 0 {
		opts = append(opts, grpc.MaxConcurrentStreams(s.cfg.MaxConcurrentStreams))
	}

	ka := s.cfg.Keepalive
	if ka.Time > 0 || ka.Timeout > 0 || ka.MaxConnectionIdle > 0 || ka.MaxConnectionAge > 0 {
		opts = append(opts, grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:              ka.Time,
			Timeout:           ka.Timeout,
			MaxConnectionIdle: ka.MaxConnectionIdle,
			MaxConnectionAge:  ka.MaxConnectionAge,
		}))
	}

	if s.cfg.TLS.Enabled {
		tlsCfg, err := s.cfg.TLS.Build()
		if err != nil {
			return nil, errutil.Explain(err, "grpc: build TLS")
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}
	return opts, nil
}

// Run starts the gRPC server after Go-Spring signals readiness.
func (s *SimpleGrpcServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	opts, err := s.buildOptions()
	if err != nil {
		return err
	}
	s.svr = grpc.NewServer(opts...)

	// Mount the standard health service so probes and load balancers can query
	// serving status via grpc_health_v1.
	if s.cfg.Health.Enabled {
		hs := health.NewServer()
		healthpb.RegisterHealthServer(s.svr, hs)
		hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	}

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
