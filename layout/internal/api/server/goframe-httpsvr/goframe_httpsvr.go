// Package goframe_httpsvr bootstraps the goframe HTTP server and binds it to
// the Go-Spring server lifecycle. It mirrors grpcsvr but adapts a goframe
// ghttp.Server to gs.Server so the container drives startup and graceful
// shutdown.
package goframe_httpsvr

import (
	"context"

	"GS_PROJECT_MODULE/idl/goframe-http/pb"
	"GS_PROJECT_MODULE/internal/api/server/goframe-httpsvr/middleware"

	"go-spring.org/spring/gs"
)

func init() {
	// Register the goframe HTTP server and bind it to the Go-Spring server
	// lifecycle. Config is filled from the ${spring.goframe.http.server}
	// prefix; the server only materialises when a pb.GS_PROJECT_NAMEServer
	// bean has been exported, mirroring how the other transport servers gate
	// on their handler bean.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.goframe.http.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[pb.GS_PROJECT_NAMEServer]())
}

// Config defines the goframe HTTP server configuration, bound from
// ${spring.goframe.http.server}.
type Config struct {
	Addr           string                    `value:"${addr:=:8004}"`
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a goframe HTTP server to the Go-Spring server lifecycle.
// ghttp.Server.Run() blocks forever, so Run offloads it to a goroutine and
// parks on a done channel; Stop closes done to hand control back to Go-Spring's
// shutdown sequence.
type Server struct {
	cfg  Config
	ctrl *GS_PROJECT_NAMEController
	done chan struct{}
}

// NewServer creates a goframe HTTP Server from ${spring.goframe.http.server}
// config and the composed controller bean.
func NewServer(cfg Config, ctrl *GS_PROJECT_NAMEController) *Server {
	return &Server{cfg: cfg, ctrl: ctrl, done: make(chan struct{})}
}

// Run builds the goframe HTTP server on the configured address, registers the
// generated service against s.ctrl, then starts serving once Go-Spring signals
// readiness.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	// TODO: build the goframe HTTP server with the middleware chain and
	// register the generated service implementation backed by s.ctrl, e.g.:
	//   srv := g.Server()
	//   srv.SetAddr(s.cfg.Addr)
	//   srv.Use(middleware.Chain(nil,
	//       middleware.Recovery(s.cfg.RecoveryConfig),
	//       middleware.Trace(s.cfg.TraceConfig),
	//       middleware.Metric(s.cfg.MetricConfig),
	//   ))
	//   pb.RegisterGS_PROJECT_NAMEServer(srv, s.ctrl)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// TODO: errCh <- srv.Run() — ghttp.Server.Run blocks until Shutdown.
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-s.done:
		return nil
	}
}

// Stop signals Run to return so Go-Spring can complete its shutdown sequence.
func (s *Server) Stop() error {
	// TODO: srv.Shutdown()
	close(s.done)
	return nil
}
