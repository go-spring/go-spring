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

// Package server adapts goframe's *ghttp.Server to the Go-Spring server
// lifecycle. In a stock goframe project this wiring lives in internal/cmd:
// g.Server() -> s.Group(...) route binding -> s.Run(). Here it is expressed as
// a gs.Server bean so the container drives startup and graceful shutdown.
package server

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go-spring.org/spring/gs"

	"go-spring.org/goframe/internal/config"
	"go-spring.org/goframe/internal/controller/hello"
)

// GoFrameServer wraps a goframe *ghttp.Server so it satisfies gs.Server.
type GoFrameServer struct {
	svr  *ghttp.Server
	done chan struct{}
}

// NewGoFrameServer builds the goframe server from the Go-Spring-bound config,
// sets its listen address explicitly (instead of letting g.Cfg() read
// manifest/config/config.yaml), and binds the same routes the scaffold
// registered in internal/cmd.
func NewGoFrameServer(cfg config.Config) *GoFrameServer {
	s := g.Server()
	s.SetAddr(cfg.Address)
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Bind(
			hello.NewV1(),
		)
	})
	return &GoFrameServer{svr: s, done: make(chan struct{})}
}

// Run starts serving once Go-Spring signals readiness. goframe's Start() is
// non-blocking (it listens in a background goroutine), so Run blocks on `done`
// until Stop is called, keeping the server bean alive for the container.
func (s *GoFrameServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	if err := s.svr.Start(); err != nil {
		return err
	}
	<-s.done
	return nil
}

// Stop gracefully shuts down the goframe server and unblocks Run. This replaces
// the process-owned signal handling that s.Run() would otherwise install.
func (s *GoFrameServer) Stop() error {
	err := s.svr.Shutdown()
	close(s.done)
	return err
}
