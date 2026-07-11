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

	"github.com/cloudwego/kitex/server"
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
	Addr string `value:"${addr:=:8888}"`
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
}

// NewKitexServer creates a KitexServer from ${spring.kitex.server} config and
// the registered EchoService handler bean.
func NewKitexServer(cfg Config, handler echo.EchoService) *KitexServer {
	return &KitexServer{cfg: cfg, handler: handler}
}

// Run builds the Kitex server on the configured address and starts serving
// once Go-Spring signals readiness. server.Run() blocks until Stop is called.
func (s *KitexServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	addr, err := net.ResolveTCPAddr("tcp", s.cfg.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to resolve addr %s", s.cfg.Addr)
	}
	s.svr = echoservice.NewServer(s.handler, server.WithServiceAddr(addr))
	<-sig.TriggerAndWait()
	if err = s.svr.Run(); err != nil {
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	}
	return nil
}

// Stop gracefully stops the underlying Kitex server.
func (s *KitexServer) Stop() error {
	return s.svr.Stop()
}
