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

// Package server adapts go-zero's rest.Server to the Go-Spring server
// lifecycle. In a stock go-zero project this wiring lives inline in main():
// rest.MustNewServer -> handler.RegisterHandlers -> server.Start(). Here it is
// expressed as a gs.Server bean so the container drives startup and shutdown.
package server

import (
	"context"
	"net/http"

	"github.com/zeromicro/go-zero/rest"
	"go-spring.org/spring/gs"

	"greet/internal/config"
	"greet/internal/handler"
	"greet/internal/svc"
)

// GreetServer wraps a go-zero rest.Server so it satisfies gs.Server.
type GreetServer struct {
	svr     *rest.Server
	httpSvr *http.Server
}

// NewGreetServer builds the go-zero rest.Server from the Go-Spring-bound
// config and registers the generated routes against the injected
// ServiceContext. It replaces the rest.MustNewServer + RegisterHandlers block
// that goctl emits in main().
func NewGreetServer(cfg config.Config, svcCtx *svc.ServiceContext) *GreetServer {
	svr := rest.MustNewServer(cfg.RestConf())
	handler.RegisterHandlers(svr, svcCtx)
	return &GreetServer{svr: svr}
}

// Run starts serving once Go-Spring signals readiness and blocks until the
// underlying HTTP server stops. StartWithOpts hands us the *http.Server before
// it begins listening, which Stop later uses for a graceful shutdown.
func (s *GreetServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	s.svr.StartWithOpts(func(svr *http.Server) {
		s.httpSvr = svr
	})
	return nil
}

// Stop gracefully shuts down the HTTP server and closes go-zero's logger.
// go-zero's own Server.Stop only flushes logs, so the explicit Shutdown here
// is what actually returns Run.
func (s *GreetServer) Stop() error {
	if s.httpSvr != nil {
		_ = s.httpSvr.Shutdown(context.Background())
	}
	s.svr.Stop()
	return nil
}
