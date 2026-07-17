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

// Package StarterKratosWs integrates a kratos-transport WebSocket server into the
// Go-Spring server lifecycle. Import it for the side effect and provide a
// ServiceRegister bean to bind message handlers:
//
//	import _ "go-spring.org/starter-kratos/ws"
//
// Configuration is bound from the ${spring.kratos.ws.server} prefix. Leave
// Etcd.Addr empty for a plain direct-connect server, or set it to publish the
// service into etcd for discovery under Name.
//
// WebSocket carries application-defined framed messages, not proto RPCs, and has
// no middleware chain — so unlike the http/grpc starters it is NOT instrumented
// with tracing/metrics. This starter also pulls in github.com/tx7do/kratos-
// transport (pinned to v1.3.1, see Run); that dependency is quarantined here so
// applications that only need HTTP or gRPC never link it.
package StarterKratosWs

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2"
	klog "github.com/go-kratos/kratos/v2/log"
	kws "github.com/tx7do/kratos-transport/transport/websocket"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"

	"go-spring.org/starter-kratos/internal/logbridge"
)

func init() {
	// Importing the starter is the opt-in; the module still guards on
	// spring.kratos.ws.server.enabled (default true). The server only
	// materializes when the application supplies a ServiceRegister bean, keeping
	// WsServer independent of any concrete message handler.
	enabled := gs.OnProperty("spring.kratos.ws.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enabled, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(NewWsServer, gs.IndexArg(0, gs.TagArg("${spring.kratos.ws.server}"))).
			Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister binds message handlers onto a kratos-transport WebSocket
// server. Extracting registration behind this function type keeps WsServer
// message-agnostic: it drives the lifecycle while each service supplies its own
// register bean, typically wrapping kws.RegisterServerMessageHandler.
type ServiceRegister func(ws *kws.Server) error

// Config binds kratos-transport WebSocket server + etcd registry configuration
// from ${spring.kratos.ws.server}.
type Config struct {
	Name    string `value:"${name:=kratos-ws}"`
	Network string `value:"${network:=}"`
	Addr    string `value:"${addr:=0.0.0.0:9002}"`
	Path    string `value:"${path:=/}"`

	// Etcd service discovery. Empty Addr disables registration (direct-connect).
	Etcd struct {
		Addr string `value:"${addr:=}"`
	} `value:"${etcd}"`
}

// WsServer adapts a kratos.App wrapping a single WebSocket transport server to
// the Go-Spring server lifecycle. The App implements startup, etcd registration
// and graceful shutdown; Go-Spring drives it alongside every other managed
// server.
type WsServer struct {
	cfg  Config
	reg  ServiceRegister
	log  klog.Logger
	app  *kratos.App
	done chan struct{}
}

// NewWsServer builds a WsServer from ${spring.kratos.ws.server} config and the
// registered ServiceRegister bean. The kratos logger bridges framework logs into
// go-spring's log module (see internal/logbridge).
func NewWsServer(cfg Config, reg ServiceRegister) *WsServer {
	return &WsServer{cfg: cfg, reg: reg, log: logbridge.NewLogger(), done: make(chan struct{})}
}

// Run builds the kratos-transport WebSocket server, composes it into a
// kratos.App together with an optional etcd Registrar, and starts serving once
// Go-Spring signals readiness. kratos.App.Run publishes the service into etcd
// (when a registrar is configured) and blocks until Stop is called, so it runs
// in a goroutine while Run parks on the done channel; Stop closes done to hand
// control back to Go-Spring after tearing the App down.
func (s *WsServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	// We use PayloadTypeBinary because kratos-transport's text-mode server has
	// an asymmetric quirk in the pinned version: it unwraps a
	// `{"type","payload"}` envelope on receive but sends back just the raw
	// codec bytes with no envelope, which forces the client to speak two
	// different formats depending on direction. Binary mode is symmetric:
	// every frame on the wire is
	//   <4-byte little-endian uint32 messageType><JSON-encoded payload bytes>
	// so the consumer can hand-craft one format and expect the same shape back.
	//
	// The kratos-transport WS dep is pinned to v1.3.1 in go.mod: v1.3.4
	// introduced a regression where the wsHandler no longer registers the
	// session with the SessionManager, so Server.SendMessage always fails with
	// "session not found" and no reply ever reaches the client. v1.3.1's
	// register-channel-based session handoff still works correctly.
	opts := []kws.ServerOption{
		kws.WithPath(s.cfg.Path),
		kws.WithCodec("json"),
		kws.WithPayloadType(kws.PayloadTypeBinary),
	}
	if s.cfg.Network != "" {
		opts = append(opts, kws.WithNetwork(s.cfg.Network))
	}
	if s.cfg.Addr != "" {
		opts = append(opts, kws.WithAddress(s.cfg.Addr))
	}
	wsSrv := kws.NewServer(opts...)

	if err := s.reg(wsSrv); err != nil {
		return errutil.Explain(err, "failed to register kratos ws service")
	}

	appOpts := []kratos.Option{
		kratos.Name(s.cfg.Name),
		kratos.Logger(s.log),
		kratos.Server(wsSrv),
	}
	// Registry turns a direct-connect setup into a real service: on Run the App
	// publishes {Name, endpoints} into etcd and Deregisters on stop. Opt-in:
	// leaving Etcd.Addr empty runs a registry-free server reached by host:port.
	if s.cfg.Etcd.Addr != "" {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{s.cfg.Etcd.Addr},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			return errutil.Explain(err, "failed to create etcd client for %s", s.cfg.Etcd.Addr)
		}
		appOpts = append(appOpts, kratos.Registrar(etcd.New(cli)))
	}
	s.app = kratos.New(appOpts...)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.app.Run()
	}()

	select {
	case err := <-errCh:
		return errutil.Explain(err, "kratos ws app exited with error")
	case <-s.done:
		return s.app.Stop()
	}
}

// Stop signals Run to tear down the kratos.App so Go-Spring can complete its
// shutdown sequence.
func (s *WsServer) Stop() error {
	close(s.done)
	return nil
}
