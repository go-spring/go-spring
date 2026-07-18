// Package trpcsvr bootstraps the tRPC server and binds it to the Go-Spring
// server lifecycle. It mirrors grpcsvr but adapts a trpc-go server to gs.Server
// so the container drives startup and graceful shutdown.
package trpcsvr

import (
	"context"

	"GS_PROJECT_MODULE/idl/trpc/pb"
	"GS_PROJECT_MODULE/internal/api/server/trpcsvr/middleware"

	"go-spring.org/spring/gs"
)

func init() {
	// Register the tRPC server and bind it to the Go-Spring server lifecycle.
	// Config is filled from the ${spring.trpc.server} prefix; the server only
	// materialises when a pb.GS_PROJECT_NAMEService bean has been exported,
	// mirroring how the grpc / dubbo / thrift starters gate on their handler
	// bean.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.trpc.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[pb.GS_PROJECT_NAMEService]())
}

// Config defines tRPC server configuration, bound from ${spring.trpc.server}.
type Config struct {
	Addr           string                    `value:"${addr:=:8100}"`
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a tRPC server to the Go-Spring server lifecycle. trpc.Server's
// Serve() blocks forever, so Run offloads it to a goroutine and parks on a
// done channel; Stop closes done to hand control back to Go-Spring's shutdown
// sequence.
type Server struct {
	cfg  Config
	ctrl *GS_PROJECT_NAMEController
	done chan struct{}
}

// NewServer creates a tRPC Server from ${spring.trpc.server} config and the
// composed controller bean.
func NewServer(cfg Config, ctrl *GS_PROJECT_NAMEController) *Server {
	return &Server{cfg: cfg, ctrl: ctrl, done: make(chan struct{})}
}

// Run builds the tRPC server on the configured address, registers the generated
// service against s.ctrl, then starts serving once Go-Spring signals readiness.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	// TODO: build the tRPC server with the filter chain and register the
	// generated service implementation backed by s.ctrl, e.g.:
	//   trpc.ServerConfigPath = ""
	//   srv := trpc.NewServer(
	//       server.WithAddress(s.cfg.Addr),
	//       server.WithFilters(middleware.ChainUnary(
	//           middleware.Recovery(s.cfg.RecoveryConfig),
	//           middleware.Trace(s.cfg.TraceConfig),
	//           middleware.Metric(s.cfg.MetricConfig),
	//       )),
	//   )
	//   pb.RegisterGS_PROJECT_NAMEService(srv, s.ctrl)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// TODO: errCh <- srv.Serve() — trpc.Server.Serve blocks until Close.
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
	// TODO: srv.Close(nil)
	close(s.done)
	return nil
}
