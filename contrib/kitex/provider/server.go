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
	"net"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	echo "go-spring.org/kitex/kitex_gen/echo"
	"go-spring.org/kitex/kitex_gen/echo/echoservice"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register the Kitex server and bind it to the Go-Spring server
	// lifecycle. Config is filled from the ${spring.kitex.server} prefix.
	// The server only materializes when an echo.EchoService bean exists,
	// mirroring how the thrift/grpc starters gate on their processor bean.
	gs.Provide(NewKitexServer, gs.IndexArg(0, gs.TagArg("${spring.kitex.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[echo.EchoService]())
}

// Config defines Kitex server configuration, bound from ${spring.kitex.server}.
type Config struct {
	Addr         string `value:"${addr:=:8888}"`
	ServiceName  string `value:"${service.name:=echo}"`
	RegistryAddr string `value:"${registry.etcd:=127.0.0.1:2379}"`
}

// KitexServer adapts a Kitex server.Server to the Go-Spring server lifecycle.
// This is the whole point of the refactor: the scaffold called svr.Run()
// directly from main(), which blocks and owns the process. Here the server
// instead implements gs.Server so Go-Spring drives startup and graceful
// shutdown alongside every other managed server.
type KitexServer struct {
	cfg     Config
	handler echo.EchoService
	svr     server.Server
	done    chan struct{}
}

// NewKitexServer creates a KitexServer from ${spring.kitex.server} config and
// the registered EchoService handler bean.
func NewKitexServer(cfg Config, handler echo.EchoService) *KitexServer {
	return &KitexServer{cfg: cfg, handler: handler, done: make(chan struct{})}
}

// Run builds the Kitex server on the configured address and starts serving
// once Go-Spring signals readiness. Serving with a registry configured makes
// Kitex publish the provider's address into etcd under its service name; the
// consumer later resolves a live provider by the same name. server.Run blocks
// forever internally, so it runs in a goroutine while Run parks on the done
// channel; Stop closes done to hand control back to Go-Spring.
func (s *KitexServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	addr, err := net.ResolveTCPAddr("tcp", s.cfg.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to resolve addr %s", s.cfg.Addr)
	}

	// Registry turns the direct-connect example into a real service: on Run
	// the provider registers itself into etcd for discovery under ServiceName.
	r, err := etcd.NewEtcdRegistry([]string{s.cfg.RegistryAddr})
	if err != nil {
		return errutil.Explain(err, "failed to create etcd registry")
	}

	s.svr = echoservice.NewServer(
		s.handler,
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
			ServiceName: s.cfg.ServiceName,
		}),
	)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// Run binds the listener, registers into etcd and then blocks.
		errCh <- s.svr.Run()
	}()

	select {
	case err = <-errCh:
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	case <-s.done:
		return nil
	}
}

// Stop gracefully stops the underlying Kitex server, deregistering it from
// etcd, and signals Run to return so Go-Spring can complete shutdown.
func (s *KitexServer) Stop() error {
	err := s.svr.Stop()
	close(s.done)
	return err
}
