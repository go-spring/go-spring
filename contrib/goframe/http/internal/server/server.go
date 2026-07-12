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
// a gs.Server bean so the container drives startup and graceful shutdown, and
// it wires goframe's built-in etcd registry so the provider publishes itself
// into etcd instead of being reached via a hard-coded host:port.
package server

import (
	"context"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/gsvc"
	"go-spring.org/spring/gs"

	"go-spring.org/goframe/http/internal/config"
	"go-spring.org/goframe/http/internal/controller/hello"
)

// GoFrameServer wraps a goframe *ghttp.Server so it satisfies gs.Server.
type GoFrameServer struct {
	svr  *ghttp.Server
	done chan struct{}
}

// NewGoFrameServer builds the goframe server from the Go-Spring-bound config,
// registers an etcd-backed gsvc.Registry globally *before* g.Server(name) is
// called (ghttp.Server snapshots gsvc.GetRegistry() at construction time), and
// binds the same routes the scaffold registered in internal/cmd. When Start is
// invoked later, ghttp will publish the service under cfg.Name into etcd; on
// Shutdown it deregisters itself.
func NewGoFrameServer(cfg config.Config) *GoFrameServer {
	// Set the global registry first so g.Server(name) picks it up as its
	// registrar. See ghttp/ghttp_server.go: `registrar: gsvc.GetRegistry()`
	// is read at server construction, so ordering matters.
	gsvc.SetRegistry(etcdreg.New(cfg.RegistryAddr))

	s := g.Server(cfg.Name)
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
// non-blocking (it listens in a background goroutine and registers the service
// into etcd), so Run blocks on `done` until Stop is called, keeping the server
// bean alive for the container.
func (s *GoFrameServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	if err := s.svr.Start(); err != nil {
		return err
	}
	<-s.done
	return nil
}

// Stop gracefully shuts down the goframe server (which also deregisters from
// etcd) and unblocks Run. This replaces the process-owned signal handling that
// s.Run() would otherwise install.
func (s *GoFrameServer) Stop() error {
	err := s.svr.Shutdown()
	close(s.done)
	return err
}
