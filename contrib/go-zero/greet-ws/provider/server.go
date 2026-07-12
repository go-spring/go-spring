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

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"go-spring.org/spring/gs"
)

func init() {
	// Register the go-zero REST server and bind it to the Go-Spring server
	// lifecycle. Config is filled from the ${spring.rest.server} prefix. The
	// server only materializes when a HandlerRegister bean exists, keeping
	// RestServer service-agnostic — same shape as the sibling greet-api.
	//
	// Why the same rest.Server hosts a WebSocket route: go-zero has no
	// dedicated WS server type. WS is served as a hand-written route on
	// rest.Server whose handler upgrades the connection. That means the
	// adapter code below is deliberately identical to greet-api's — only
	// what the HandlerRegister does inside changes.
	gs.Provide(NewRestServer, gs.IndexArg(0, gs.TagArg("${spring.rest.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[HandlerRegister]())
}

// HandlerRegister registers handlers onto a *rest.Server. Extracting the
// registration behind this function type keeps RestServer independent of any
// concrete route table; each service supplies its own register bean.
type HandlerRegister func(server *rest.Server)

// Config defines go-zero REST server configuration, bound from
// ${spring.rest.server}. Same fields as greet-api's Config — WebSocket rides
// on the same rest.RestConf, there are no extra WS-specific knobs.
type Config struct {
	Name string `value:"${name:=greet-ws}"`
	Host string `value:"${host:=0.0.0.0}"`
	Port int    `value:"${port:=8890}"`
}

// RestServer adapts a go-zero rest.Server to the Go-Spring server lifecycle.
// Identical pattern to the sibling greet-api: rest.Server.Start blocks
// internally, so it runs in a goroutine while Run parks on a done channel,
// and Stop closes done to trigger rest.Server.Stop.
type RestServer struct {
	cfg  Config
	reg  HandlerRegister
	svr  *rest.Server
	done chan struct{}
}

// NewRestServer builds a RestServer from ${spring.rest.server} config and
// the registered HandlerRegister bean.
func NewRestServer(cfg Config, reg HandlerRegister) *RestServer {
	return &RestServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// Run builds the rest.Server on the configured Host:Port, hands it to the
// HandlerRegister bean to attach routes (including the WS upgrade route),
// and then serves once Go-Spring signals readiness.
func (s *RestServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	rc := rest.RestConf{
		ServiceConf: service.ServiceConf{Name: s.cfg.Name},
		Host:        s.cfg.Host,
		Port:        s.cfg.Port,
	}
	s.svr = rest.MustNewServer(rc)
	s.reg(s.svr)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
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
func (s *RestServer) Stop() error {
	close(s.done)
	return nil
}
