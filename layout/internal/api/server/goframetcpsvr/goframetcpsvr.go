// Package goframetcpsvr bootstraps a dedicated raw-TCP server and binds it to
// the Go-Spring server lifecycle. It owns its own net.Listener on an
// independent port (${spring.goframe.tcp.server}) — there is no request-
// response IDL surface, only a frame codec agreed with the client (see
// idl/goframe-tcp/payload.proto). This package mirrors wssvr's own-listener
// pattern: the connection handler and its middleware chain live here in the
// server package, so there is no controller matrix in api/controller.
package goframetcpsvr

import (
	"context"
	"errors"
	"net"

	"GS_PROJECT_MODULE/internal/api/server/goframetcpsvr/middleware"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

func init() {
	// Register the goframe TCP server and bind it to the Go-Spring server
	// lifecycle. Config is filled from ${spring.goframe.tcp.server}; unlike
	// the IDL servers there is no handler bean to gate on — the connection
	// handler lives inside this package.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.goframe.tcp.server}"))).
		Export(gs.As[gs.Server]())
}

// Config defines the goframe TCP server configuration, bound from
// ${spring.goframe.tcp.server}. Addr is the independent listen address; the
// per-connection middleware chain is tuned via the sub-configs.
type Config struct {
	Addr           string                    `value:"${addr:=:9200}"`
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a raw-TCP accept loop to the Go-Spring server lifecycle.
type Server struct {
	cfg  Config
	ctrl *GS_PROJECT_NAMETCPController
	ln   net.Listener
	done chan struct{}
}

// NewServer creates a goframe TCP Server from ${spring.goframe.tcp.server}
// config and the connection controller bean.
func NewServer(cfg Config, ctrl *GS_PROJECT_NAMETCPController) *Server {
	return &Server{cfg: cfg, ctrl: ctrl, done: make(chan struct{})}
}

// Run opens the listener on the configured address and starts accepting once
// Go-Spring signals readiness. Each accepted connection is dispatched through
// the middleware chain to serve, which owns the framing/dispatch loop.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	s.ln = ln

	serve := middleware.Chain(s.ctrl.Serve,
		middleware.Recovery(s.cfg.RecoveryConfig),
		middleware.Trace(s.cfg.TraceConfig),
		middleware.Metric(s.cfg.MetricConfig),
	)

	<-sig.TriggerAndWait()

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Errorf(ctx, log.TagAppDef, "failed to accept goframe tcp conn: %v", err)
			return err
		}
		go serve(ctx, conn)
	}
}

// Stop closes the listener so the accept loop returns and Go-Spring can
// complete its shutdown sequence.
func (s *Server) Stop() error {
	close(s.done)
	if s.ln == nil {
		return nil
	}
	return s.ln.Close()
}
