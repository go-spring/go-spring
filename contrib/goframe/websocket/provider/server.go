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

// Package main hosts the provider binary. server.go adapts goframe's
// *ghttp.Server to the Go-Spring server lifecycle for the WebSocket example.
//
// Why the *HTTP* server appears in a "websocket" module: goframe has no
// standalone WebSocket server type. A WebSocket connection starts life as an
// HTTP GET whose "Upgrade" header the server turns into a persistent
// bidirectional frame stream. In goframe that upgrade is a one-liner inside a
// normal ghttp handler via r.WebSocket(), which internally uses
// gorilla/websocket. As a result:
//
//   - The listener, the etcd registration and the "service" identity are all
//     the same primitives as the sibling `../http` module — routes, gsvc,
//     ghttp.Server.Start/Shutdown.
//   - The consumer discovers an HTTP endpoint from etcd exactly like the http
//     sibling does, then dials ws:// against that endpoint. There is no
//     separate ws://<service-name> discovery layer in goframe today; the
//     protocol upgrade happens *after* the transport address is resolved.
//
// This adapter therefore reads almost the same as the http sibling; the only
// substantive difference lives in the /echo handler (see handler.go), which
// upgrades and echoes frames instead of writing a response body.
package main

import (
	"context"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/gsvc"
	"go-spring.org/spring/gs"
)

func init() {
	// The goframe *ghttp.Server, exported as a gs.Server so the Go-Spring
	// lifecycle starts and stops it. Config is bound from the
	// "${goframe.websocket}" prefix.
	//
	// WebSocket in goframe is not a separate server: the same *ghttp.Server
	// that would answer HTTP requests upgrades to WebSocket on the /echo
	// route (see the handler in handler.go). Registration into etcd via gsvc
	// happens at HTTP-server granularity, which is why the WS variant still
	// ends up using the exact same lifecycle bean as the http sibling.
	gs.Provide(NewGoFrameServer, gs.IndexArg(0, gs.TagArg("${goframe.websocket}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[ServiceRegister]())
}

// ServiceRegister binds routes onto the raw *ghttp.Server that GoFrameServer
// wraps. This function type keeps the adapter service-agnostic: it drives the
// listen/register lifecycle while each service supplies its own route bean
// (here the /echo WebSocket upgrade). Mirrors the grpc/http/tcp siblings —
// same name, same "bind onto the native server I hand you" shape.
type ServiceRegister func(s *ghttp.Server)

// Config holds the goframe WebSocket server settings.
//
// WebSocket in goframe is not a distinct server type: the *ghttp.Server owns
// the listener, and any HTTP route can upgrade the connection to WebSocket
// via ghttp.Request.WebSocket() (which wraps gorilla/websocket underneath).
// That is why the fields here mirror the sibling `../http` module almost
// verbatim — the transport differs, but the config surface (bind address,
// service name, etcd address) is the same because gsvc registration hangs
// off the HTTP server.
//
// Values come from Go-Spring properties (see conf/app.properties) under the
// "${goframe.websocket}" prefix using `value` tags, instead of goframe's own
// manifest/config/config.yaml loader.
type Config struct {
	Address      string `value:"${address:=:8002}"`
	Name         string `value:"${name:=goframe.websocket.echo}"`
	RegistryAddr string `value:"${registry.etcd:=127.0.0.1:2379}"`
}

// GoFrameServer wraps a goframe *ghttp.Server so it satisfies gs.Server.
type GoFrameServer struct {
	svr  *ghttp.Server
	done chan struct{}
}

// NewGoFrameServer builds the goframe server from the Go-Spring-bound config,
// registers an etcd-backed gsvc.Registry globally *before* g.Server(name) is
// called (ghttp.Server snapshots gsvc.GetRegistry() at construction time), and
// binds routes via the injected ServiceRegister (see handler.go — the /echo
// route that upgrades to WebSocket and echoes frames). When Start is invoked
// later, ghttp will publish the service under cfg.Name into etcd; on Shutdown
// it deregisters itself.
func NewGoFrameServer(cfg Config, reg ServiceRegister) *GoFrameServer {
	// Route goframe's own glog logs into go-spring's log module (see
	// logbridge.go). Installed before g.Server(name) so ghttp lifecycle and
	// WebSocket upgrade errors flow through the same pipeline as the business
	// logs.
	installGoFrameLogBridge()

	// Set the global registry first so g.Server(name) picks it up as its
	// registrar. See ghttp/ghttp_server.go: `registrar: gsvc.GetRegistry()`
	// is read at server construction, so ordering matters. This mirrors the
	// http sibling exactly — the WebSocket transport rides on the same
	// gsvc-enabled ghttp.Server.
	gsvc.SetRegistry(etcdreg.New(cfg.RegistryAddr))

	s := g.Server(cfg.Name)
	s.SetAddr(cfg.Address)
	reg(s)

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
