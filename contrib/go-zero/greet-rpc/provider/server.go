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
type Config struct {
	ListenOn string `value:"${listen-on:=0.0.0.0:8081}"`
	EtcdAddr string `value:"${etcd.addr:=127.0.0.1:2379}"`
	EtcdKey  string `value:"${etcd.key:=greet.rpc}"`
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
