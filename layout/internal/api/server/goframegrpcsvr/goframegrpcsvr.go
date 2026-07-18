// Package goframegrpcsvr bootstraps the goframe gRPC server and binds it to
// the Go-Spring server lifecycle. It mirrors grpcsvr but targets the goframe
// grpcx server so the container drives startup and graceful shutdown.
package goframegrpcsvr

import (
	"context"

	"GS_PROJECT_MODULE/idl/goframe-grpc/pb"
	"GS_PROJECT_MODULE/internal/api/server/goframegrpcsvr/middleware"

	"go-spring.org/spring/gs"
)

func init() {
	// Register the goframe gRPC server and bind it to the Go-Spring server
	// lifecycle. Config is filled from the ${spring.goframe.grpc.server}
	// prefix; the server only materialises when a pb.GS_PROJECT_NAMEServer
	// bean has been exported, mirroring how grpcsvr gates on its handler bean.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.goframe.grpc.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[pb.GS_PROJECT_NAMEServer]())
}

// Config defines the goframe gRPC server configuration, bound from
// ${spring.goframe.grpc.server}.
type Config struct {
	Addr           string                    `value:"${addr:=:9093}"`
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a goframe gRPC server to the Go-Spring server lifecycle. The
// goframe grpcx server's Run() blocks forever, so Run offloads it to a
// goroutine and parks on a done channel; Stop closes done to hand control back
// to Go-Spring's shutdown sequence.
type Server struct {
	cfg  Config
	ctrl *GS_PROJECT_NAMEController
	done chan struct{}
}

// NewServer creates a goframe gRPC Server from ${spring.goframe.grpc.server}
// config and the composed controller bean.
func NewServer(cfg Config, ctrl *GS_PROJECT_NAMEController) *Server {
	return &Server{cfg: cfg, ctrl: ctrl, done: make(chan struct{})}
}

// Run builds the goframe gRPC server on the configured address, registers the
// generated service against s.ctrl, then starts serving once Go-Spring signals
// readiness.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	// TODO: build the goframe gRPC server with the interceptor chain and
	// register the generated service implementation backed by s.ctrl, e.g.:
	//   srv := grpcx.Server.New(&grpcx.ServerConfig{
	//       Address: s.cfg.Addr,
	//       UnaryInterceptors: middleware.ChainUnary(
	//           middleware.Recovery(s.cfg.RecoveryConfig),
	//           middleware.Trace(s.cfg.TraceConfig),
	//           middleware.Metric(s.cfg.MetricConfig),
	//       ),
	//   })
	//   pb.RegisterGS_PROJECT_NAMEServer(srv, s.ctrl)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// TODO: errCh <- srv.Run() — grpcx.Server.Run blocks until Stop.
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
	// TODO: srv.Stop()
	close(s.done)
	return nil
}
