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
	consul "github.com/kitex-contrib/registry-consul"
	"go-spring.org/registry/consul/idl/echo/echoservice"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register the Kitex server as a gs.Server so Go-Spring drives its lifecycle.
	// Unlike the etcd example (which uses starter-kitex), consul is wired here in
	// the example because starter-kitex only knows how to build an etcd registry —
	// so this file plays the role starter-kitex's SimpleKitexServer would, but with
	// a consul registry.
	gs.Provide(&KitexServer{}).Export(gs.As[gs.Server]())
}

// KitexServer adapts a Kitex server.Server to the Go-Spring server lifecycle and
// publishes the service into Consul for discovery.
type KitexServer struct {
	// Addr is the Kitex bind address. It must be a concrete host:port (not a
	// wildcard) because Consul registers exactly this address and health-checks it
	// over TCP; a provider that advertised 0.0.0.0 could never pass the check.
	Addr string `value:"${spring.kitex.server.addr:=127.0.0.1:8888}"`
	// ServiceName is the name published into Consul; the consumer resolves by it.
	ServiceName string `value:"${spring.kitex.server.service.name:=echo}"`
	// RegistryAddr is the Consul agent HTTP address (host:port).
	RegistryAddr string `value:"${spring.kitex.server.registry.consul:=127.0.0.1:8500}"`

	svr  server.Server
	done chan struct{}
}

// Run builds the Kitex server, attaches a Consul registry so the provider
// publishes itself under ServiceName on startup, then serves until Stop.
func (s *KitexServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	addr, err := net.ResolveTCPAddr("tcp", s.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to resolve addr %s", s.Addr)
	}

	r, err := consul.NewConsulRegister(s.RegistryAddr)
	if err != nil {
		return errutil.Explain(err, "failed to create consul registry")
	}

	s.done = make(chan struct{})
	s.svr = server.NewServer(
		server.WithServiceAddr(addr),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: s.ServiceName}),
		server.WithRegistry(r),
	)
	if err = echoservice.RegisterService(s.svr, &EchoServiceImpl{}); err != nil {
		return errutil.Explain(err, "failed to register service")
	}

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() { errCh <- s.svr.Run() }()

	select {
	case err = <-errCh:
		return errutil.Explain(err, "failed to serve on %s", s.Addr)
	case <-s.done:
		return nil
	}
}

// Stop gracefully stops the Kitex server, deregistering it from Consul, and
// signals Run to return.
func (s *KitexServer) Stop() error {
	err := s.svr.Stop()
	close(s.done)
	return err
}
