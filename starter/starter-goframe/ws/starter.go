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

// Package StarterGoFrameWS integrates goframe's WebSocket support into the
// Go-Spring server lifecycle. goframe has no standalone WebSocket server type: a
// WebSocket connection starts as an HTTP GET whose "Upgrade" header a normal
// ghttp handler turns into a frame stream via r.WebSocket() (backed by
// gorilla/websocket). So this starter wraps the same *ghttp.Server as the http
// sibling; the difference lives entirely in the handler the application binds.
//
// Import it for the side effect and provide a ServiceRegister bean:
//
//	import _ "go-spring.org/starter-goframe/ws"
//
// Configuration is bound from the ${spring.goframe.ws.server} prefix.
package StarterGoFrameWS

import (
	"context"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/gsvc"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"

	"go-spring.org/starter-goframe/internal/logbridge"
)

func init() {
	// Importing the starter is the opt-in; the module still guards on
	// spring.goframe.ws.server.enabled (default true). The *ghttp.Server only
	// materialises when the application supplies a ServiceRegister bean, keeping
	// WSServer service-agnostic — each service binds its own upgrade route.
	enabled := gs.OnProperty("spring.goframe.ws.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enabled, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(NewWSServer, gs.IndexArg(0, gs.TagArg("${spring.goframe.ws.server}"))).
			Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister binds routes onto the raw *ghttp.Server that WSServer wraps.
// This function type keeps the adapter service-agnostic: it drives the
// listen/register lifecycle while each service supplies its own route bean
// (typically an /echo-style route that calls r.WebSocket() to upgrade). The
// param is the raw server (not a router group) because a WebSocket upgrade route
// must not sit under goframe's response-wrapping middleware.
type ServiceRegister func(s *ghttp.Server)

// Config binds goframe WebSocket server settings from ${spring.goframe.ws.server}.
//
// WebSocket in goframe is not a distinct server type: the *ghttp.Server owns the
// listener, and any HTTP route can upgrade the connection. That is why these
// fields mirror the http sibling — gsvc registration hangs off the HTTP server.
type Config struct {
	Name    string `value:"${name:=goframe}"`
	Address string `value:"${address:=:8002}"`

	// Registry publishes the server into etcd for discovery. Leave etcd empty
	// (the default) for a plain server with no registration.
	Registry struct {
		Etcd string `value:"${etcd:=}"`
	} `value:"${registry}"`
}

// WSServer wraps a goframe *ghttp.Server so it satisfies gs.Server.
type WSServer struct {
	svr  *ghttp.Server
	done chan struct{}
}

// NewWSServer builds the goframe server from the Go-Spring-bound config,
// optionally registers an etcd-backed gsvc.Registry globally *before*
// g.Server(name) is called (ghttp snapshots gsvc.GetRegistry() at construction),
// and binds the upgrade route via the injected ServiceRegister.
func NewWSServer(cfg Config, reg ServiceRegister) *WSServer {
	// Route goframe's own glog logs into go-spring's log module. Installed
	// before g.Server(name) so ghttp lifecycle and WebSocket upgrade errors flow
	// through the same pipeline as the business logs.
	logbridge.Install()

	// Set the global registry first so g.Server(name) picks it up as its
	// registrar. Ordering matters — ghttp reads it at construction. Skipped when
	// unconfigured.
	if cfg.Registry.Etcd != "" {
		gsvc.SetRegistry(etcdreg.New(cfg.Registry.Etcd))
	}

	svr := g.Server(cfg.Name)
	svr.SetAddr(cfg.Address)
	reg(svr)

	return &WSServer{svr: svr, done: make(chan struct{})}
}

// Run starts serving once Go-Spring signals readiness. goframe's Start() is
// non-blocking, so Run blocks on `done` until Stop is called.
func (s *WSServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	if err := s.svr.Start(); err != nil {
		return err
	}
	<-s.done
	return nil
}

// Stop gracefully shuts down the goframe server (which also deregisters from
// etcd when a registry is set) and unblocks Run.
func (s *WSServer) Stop() error {
	err := s.svr.Shutdown()
	close(s.done)
	return err
}
