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

// Package server adapts goframe's *gtcp.Server to the Go-Spring server
// lifecycle and manually wires it into gsvc for etcd registration.
//
// Why this module looks so different from ../http and ../grpc:
//
//   - ghttp.Server and grpcx.GrpcServer both integrate with gsvc out of the
//     box: they snapshot gsvc.GetRegistry() at construction time and call
//     Register/Deregister for you as part of Start/Shutdown.
//   - gtcp.Server has no such integration. It is a plain listen+Accept loop
//     with a user-supplied handler; it does not know about gsvc, service
//     names, or etcd.
//
// So for a raw TCP transport the "goframe pattern" is: build the gtcp
// server, and then call gsvc.Registrar.Register / Deregister by hand around
// its lifetime. That is exactly what this adapter does — it is a worked
// example of using gsvc's *primitives* on a transport goframe has not
// pre-wired.
//
// The consumer side mirrors it: gsvc.Discovery.Search yields a live
// endpoint, and then gtcp.NewConn dials it directly (no framework client to
// mediate).
package server

import (
	"bufio"
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

	"go-spring.org/goframe/tcp/internal/config"
)

// GoFrameTCPServer wraps a goframe *gtcp.Server together with the gsvc
// registration lifecycle so it satisfies gs.Server.
type GoFrameTCPServer struct {
	cfg      config.Config
	svr      *gtcp.Server
	registry gsvc.Registry
	// registered is the Service returned by the registrar after a successful
	// Register call. Deregister must be called with the same object because
	// the etcd registrar keeps the lease keyed off it.
	registered gsvc.Service
	// done is closed by Stop() to unblock Run() after the server bean has
	// been shut down.
	done chan struct{}
	// runErr carries any error the blocking gtcp.Server.Run() returns so
	// Run() can surface it back to Go-Spring.
	runErr chan error
	// stopping is flipped by Stop() before it closes the listener so the
	// Run-goroutine can filter the expected "use of closed network
	// connection" error out of the returned err.
	stopping atomic.Bool
}

// NewGoFrameTCPServer builds the gtcp.Server from the Go-Spring-bound config
// and pre-creates (but does not yet call) the etcd registry. Actual
// registration happens in Run once the listener is up, so the endpoint we
// publish is guaranteed to be reachable — publishing before Listen would
// race consumers into a "connection refused" window.
//
// The handler is a bufio.Reader-backed line echo: for every newline-
// terminated frame we receive, we write the same bytes back. That gives the
// consumer a deterministic value to assert on.
func NewGoFrameTCPServer(cfg config.Config) *GoFrameTCPServer {
	handler := func(conn *gtcp.Conn) {
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			// ReadBytes preserves the delimiter so the echoed frame is
			// self-describing on the wire; the consumer trims it.
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				if _, werr := conn.Write(line); werr != nil {
					return
				}
			}
			if err != nil {
				// io.EOF or a broken pipe both end the loop; the deferred
				// Close will run.
				return
			}
		}
	}
	s := gtcp.NewServer(cfg.Address, handler)

	return &GoFrameTCPServer{
		cfg:      cfg,
		svr:      s,
		registry: etcdreg.New(cfg.RegistryAddr),
		done:     make(chan struct{}),
		runErr:   make(chan error, 1),
	}
}

// Run starts serving once Go-Spring signals readiness, publishes the service
// into etcd, then parks until Stop() closes `done`. gtcp.Server.Run() is
// blocking (it owns the Accept loop), so we run it in its own goroutine and
// forward any error out through runErr.
//
// Note the ordering: bind first, register second. Registering before the
// listener is up would let consumers dial into a not-yet-listening port.
func (s *GoFrameTCPServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()

	go func() {
		// gtcp.Server.Run only returns on error or after Close(); either
		// way we send once so Run() can pick it up. When Stop() has flagged
		// us as shutting down, the "use of closed network connection"
		// error from Accept is expected — swallow it so Go-Spring does not
		// log a fake shutdown failure. (net.ErrClosed matches; some
		// wrapped forms carry the string only, hence the fallback check.)
		err := s.svr.Run()
		if err != nil && s.stopping.Load() {
			if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection") {
				err = nil
			}
		}
		s.runErr <- err
	}()

	// Give the listener a moment to become ready. gtcp.Server does not
	// expose a "listening" signal, so this poll-and-sleep is the least bad
	// way to avoid racing the registration with the Accept loop.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s.svr.GetListenedPort() > 0 {
			break
		}
		select {
		case err := <-s.runErr:
			// The server failed to start; surface the error immediately.
			return err
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Register under the advertised host:port. We use LocalService directly
	// (rather than NewServiceWithName) so we can attach the endpoint the
	// clients should dial; NewServiceWithName leaves Endpoints empty.
	svc := &gsvc.LocalService{
		Name:      s.cfg.Name,
		Endpoints: gsvc.Endpoints{gsvc.NewEndpoint(endpointAddr(s.cfg))},
	}
	registered, err := s.registry.Register(context.Background(), svc)
	if err != nil {
		return err
	}
	s.registered = registered

	// Park until Stop() is called; also surface a late Run() error.
	select {
	case <-s.done:
		return nil
	case err := <-s.runErr:
		return err
	}
}

// Stop deregisters from etcd and closes the gtcp.Server listener. Order
// matters: deregister first so no new consumers pick this instance up while
// its listener is still shutting down. Setting `stopping` before Close()
// lets the Run goroutine treat the resulting Accept error as expected
// shutdown rather than a real serve failure.
func (s *GoFrameTCPServer) Stop() error {
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

// endpointAddr builds the "host:port" string the provider advertises. Kept
// as a small helper so the Config type stays a pure value object.
func endpointAddr(cfg config.Config) string {
	return net.JoinHostPort(cfg.AdvertiseHost, strconv.Itoa(cfg.AdvertisePort))
}
