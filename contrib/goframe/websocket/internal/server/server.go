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
// lifecycle for the WebSocket example.
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
// substantive difference lives in the /echo handler, which upgrades and
// echoes frames instead of writing a response body.
package server

import (
	"context"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/gsvc"
	"go-spring.org/spring/gs"

	"go-spring.org/goframe/websocket/internal/config"
)

// GoFrameServer wraps a goframe *ghttp.Server so it satisfies gs.Server.
type GoFrameServer struct {
	svr  *ghttp.Server
	done chan struct{}
}

// NewGoFrameServer builds the goframe server from the Go-Spring-bound config,
// registers an etcd-backed gsvc.Registry globally *before* g.Server(name) is
// called (ghttp.Server snapshots gsvc.GetRegistry() at construction time), and
// binds a single /echo route that upgrades to WebSocket and echoes frames.
// When Start is invoked later, ghttp will publish the service under cfg.Name
// into etcd; on Shutdown it deregisters itself.
func NewGoFrameServer(cfg config.Config) *GoFrameServer {
	// Set the global registry first so g.Server(name) picks it up as its
	// registrar. See ghttp/ghttp_server.go: `registrar: gsvc.GetRegistry()`
	// is read at server construction, so ordering matters. This mirrors the
	// http sibling exactly — the WebSocket transport rides on the same
	// gsvc-enabled ghttp.Server.
	gsvc.SetRegistry(etcdreg.New(cfg.RegistryAddr))

	s := g.Server(cfg.Name)
	s.SetAddr(cfg.Address)

	// A plain ghttp handler that promotes the connection to WebSocket. This
	// is the *entire* transport difference vs the http sibling: instead of
	// writing a response body, we call r.WebSocket() to swap the HTTP
	// connection for a gorilla-backed WebSocket, then run a read/echo loop
	// on it. Any HTTP-level middleware, timeouts and gsvc registration
	// continue to apply to the initial handshake request.
	s.BindHandler("/echo", func(r *ghttp.Request) {
		ws, err := r.WebSocket()
		if err != nil {
			// Handshake failed (bad headers, wrong method, etc). ghttp
			// has already written a 4xx; nothing more to do.
			return
		}
		defer ws.Close()
		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				// Client closed the connection or the network broke; end
				// the echo loop. ws.Close() runs via defer.
				return
			}
			// Echo the frame back verbatim — same message type, same
			// payload — giving the consumer a deterministic value to
			// assert on.
			if err := ws.WriteMessage(msgType, data); err != nil {
				return
			}
		}
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
