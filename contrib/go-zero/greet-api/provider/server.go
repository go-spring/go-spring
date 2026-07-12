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
	// RestServer service-agnostic — the same pattern the sibling greet-rpc
	// uses for its ServiceRegister bean.
	gs.Provide(NewRestServer, gs.IndexArg(0, gs.TagArg("${spring.rest.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[HandlerRegister]())
}

// HandlerRegister registers handlers onto a *rest.Server. Extracting the
// registration behind this function type keeps RestServer independent of any
// concrete generated route table; each service supplies its own register bean.
type HandlerRegister func(server *rest.Server)

// Config defines go-zero REST server configuration, bound from
// ${spring.rest.server}.
//
// go-zero's rest.RestConf embeds service.ServiceConf and adds Host+Port and a
// pile of optional knobs (timeouts, TLS, telemetry, …). This example only
// surfaces the three fields the stock etc/*.yaml would carry — Name, Host,
// Port — and lets go-zero fill the rest with defaults.
type Config struct {
	Name string `value:"${name:=greet}"`
	Host string `value:"${host:=0.0.0.0}"`
	Port int    `value:"${port:=8888}"`
}

// RestServer adapts a go-zero rest.Server to the Go-Spring server lifecycle.
// The stock go-zero pattern is `srv.Start()` blocking in main(); here the
// server instead implements gs.Server so Go-Spring drives startup and
// graceful shutdown alongside every other managed server.
type RestServer struct {
	cfg  Config
	reg  HandlerRegister
	svr  *rest.Server
	done chan struct{}
}

// NewRestServer builds a RestServer from ${spring.rest.server} config and the
// registered HandlerRegister bean.
func NewRestServer(cfg Config, reg HandlerRegister) *RestServer {
	return &RestServer{cfg: cfg, reg: reg, done: make(chan struct{})}
}

// Run builds the rest.Server on the configured Host:Port, hands it to the
// HandlerRegister bean to attach routes, and then serves once Go-Spring
// signals readiness. rest.Server.Start blocks internally, so it runs in a
// goroutine while Run parks on the done channel; Stop closes done to hand
// control back to Go-Spring.
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
		// Start binds the listener and blocks until Stop is called.
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
