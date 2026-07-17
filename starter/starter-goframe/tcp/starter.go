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

// Package StarterGoFrameTCP integrates goframe's *gtcp.Server into the
// Go-Spring server lifecycle. Import it for the side effect and provide a
// ServiceRegister bean to attach a connection handler:
//
//	import _ "go-spring.org/starter-goframe/tcp"
//
// Configuration is bound from the ${spring.goframe.tcp.server} prefix.
//
// Unlike ghttp.Server / grpcx.GrpcServer, gtcp has no built-in gsvc integration
// — it is a plain listen+Accept loop. When an etcd address is configured this
// adapter performs Register / Deregister by hand around the listener lifetime;
// leave it empty for a plain TCP server with no discovery.
package StarterGoFrameTCP

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	etcdreg "github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/v2/net/gsvc"
	"github.com/gogf/gf/v2/net/gtcp"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"

	"go-spring.org/starter-goframe/internal/logbridge"
)

func init() {
	// Importing the starter is the opt-in; the module still guards on
	// spring.goframe.tcp.server.enabled (default true). The gtcp.Server only
	// materialises when the application supplies a ServiceRegister bean, keeping
	// TCPServer service-agnostic — each service supplies its own handler.
	enabled := gs.OnProperty("spring.goframe.tcp.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enabled, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(NewTCPServer, gs.IndexArg(0, gs.TagArg("${spring.goframe.tcp.server}"))).
			Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[ServiceRegister]())
		return nil
	})
}

// ServiceRegister binds a connection handler onto the raw *gtcp.Server that
// TCPServer wraps (via s.SetHandler). This function type keeps the adapter
// service-agnostic: it drives the listen/register lifecycle while each service
// supplies its own handler bean.
type ServiceRegister func(s *gtcp.Server)

// Config binds goframe raw-TCP server settings from ${spring.goframe.tcp.server}.
//
// gtcp has no notion of gsvc: unlike ghttp.Server / grpcx.GrpcServer, it does
// not snapshot a Registry at construction and does not publish itself on Start.
// Registration into etcd is therefore done by hand (see Run), which is why this
// Config carries an explicit AdvertiseHost/Port — with gtcp there is no
// framework-side "detect my outbound IP" step to fall back on. Registration only
// happens when Registry.Etcd is set.
type Config struct {
	Name string `value:"${name:=goframe}"`

	// Address is the gtcp.Server bind address, e.g. ":8003".
	Address string `value:"${address:=:8003}"`

	// AdvertiseHost/Port is the endpoint published into etcd. Because gtcp binds
	// on Address and never asks the OS for a public IP, the consumer would fail
	// to dial "0.0.0.0" — the provider has to name the address clients connect
	// on. Only used when Registry.Etcd is set.
	AdvertiseHost string `value:"${advertise.host:=127.0.0.1}"`
	AdvertisePort int    `value:"${advertise.port:=8003}"`

	// Registry publishes the server into etcd for discovery. Leave etcd empty
	// (the default) for a plain TCP server with no registration.
	Registry struct {
		Etcd string `value:"${etcd:=}"`
	} `value:"${registry}"`
}

// TCPServer wraps a goframe *gtcp.Server together with the optional gsvc
// registration lifecycle so it satisfies gs.Server.
type TCPServer struct {
	cfg      Config
	svr      *gtcp.Server
	registry gsvc.Registry
	// registered is the Service returned by the registrar after a successful
	// Register call. Deregister must be called with the same object because the
	// etcd registrar keeps the lease keyed off it. Nil when registration is off.
	registered gsvc.Service
	// done is closed by Stop() to unblock Run() after shutdown.
	done chan struct{}
	// runErr carries any error the blocking gtcp.Server.Run() returns so Run()
	// can surface it back to Go-Spring.
	runErr chan error
	// stopping is flipped by Stop() before it closes the listener so the
	// Run-goroutine can filter the expected "use of closed network connection"
	// error out of the returned err.
	stopping atomic.Bool
}

// NewTCPServer builds the gtcp.Server from the Go-Spring-bound config and, when
// an etcd address is set, pre-creates (but does not yet call) the registry.
// Actual registration happens in Run once the listener is up. The connection
// handler is bound by the injected ServiceRegister, so this adapter never names
// a concrete service.
func NewTCPServer(cfg Config, reg ServiceRegister) *TCPServer {
	// Route goframe's own glog logs into go-spring's log module. Installed
	// before the listener is built so registration errors and other startup
	// lines flow through the shared pipeline.
	logbridge.Install()

	// Build the server with a nil handler, then let the injected register bean
	// attach the service handler via SetHandler — keeping the concrete service
	// out of this adapter.
	s := gtcp.NewServer(cfg.Address, nil)
	reg(s)

	t := &TCPServer{
		cfg:    cfg,
		svr:    s,
		done:   make(chan struct{}),
		runErr: make(chan error, 1),
	}
	if cfg.Registry.Etcd != "" {
		t.registry = etcdreg.New(cfg.Registry.Etcd)
	}
	return t
}

// Run starts serving once Go-Spring signals readiness, publishes the service
// into etcd (when a registry is configured), then parks until Stop() closes
// `done`. gtcp.Server.Run() is blocking (it owns the Accept loop), so we run it
// in its own goroutine and forward any error out through runErr.
//
// Note the ordering: bind first, register second. Registering before the
// listener is up would let consumers dial into a not-yet-listening port.
func (s *TCPServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()

	go func() {
		// gtcp.Server.Run only returns on error or after Close(); either way we
		// send once so Run() can pick it up. When Stop() has flagged us as
		// shutting down, the "use of closed network connection" error from
		// Accept is expected — swallow it so Go-Spring does not log a fake
		// shutdown failure.
		err := s.svr.Run()
		if err != nil && s.stopping.Load() {
			if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection") {
				err = nil
			}
		}
		s.runErr <- err
	}()

	if s.registry == nil {
		// No discovery: just park until Stop, surfacing a late serve error.
		select {
		case <-s.done:
			return nil
		case err := <-s.runErr:
			return err
		}
	}

	// Give the listener a moment to become ready. gtcp.Server does not expose a
	// "listening" signal, so this poll-and-sleep is the least bad way to avoid
	// racing the registration with the Accept loop.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s.svr.GetListenedPort() > 0 {
			break
		}
		select {
		case err := <-s.runErr:
			return err
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Register under the advertised host:port. We use LocalService directly
	// (rather than NewServiceWithName) so we can attach the endpoint the clients
	// should dial; NewServiceWithName leaves Endpoints empty.
	svc := &gsvc.LocalService{
		Name:      s.cfg.Name,
		Endpoints: gsvc.Endpoints{gsvc.NewEndpoint(endpointAddr(s.cfg))},
	}
	registered, err := s.registry.Register(context.Background(), svc)
	if err != nil {
		return err
	}
	s.registered = registered

	select {
	case <-s.done:
		return nil
	case err := <-s.runErr:
		return err
	}
}

// Stop deregisters from etcd (when registered) and closes the gtcp.Server
// listener. Order matters: deregister first so no new consumers pick this
// instance up while its listener is still shutting down. Setting `stopping`
// before Close() lets the Run goroutine treat the resulting Accept error as
// expected shutdown rather than a real serve failure.
func (s *TCPServer) Stop() error {
	if s.registered != nil {
		// Best-effort deregister; if etcd is already gone there is nothing
		// useful the caller can do with the error.
		_ = s.registry.Deregister(context.Background(), s.registered)
	}
	s.stopping.Store(true)
	err := s.svr.Close()
	close(s.done)
	return err
}

// endpointAddr builds the "host:port" string the provider advertises.
func endpointAddr(cfg Config) string {
	return net.JoinHostPort(cfg.AdvertiseHost, strconv.Itoa(cfg.AdvertisePort))
}
