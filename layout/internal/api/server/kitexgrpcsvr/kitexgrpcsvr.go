// Package kitexgrpcsvr bootstraps the Kitex gRPC/protobuf server and binds it
// to the Go-Spring server lifecycle. It mirrors httpsvr but adapts a Kitex
// server to gs.Server so the container drives startup and graceful shutdown.
package kitexgrpcsvr

import (
	"context"

	svc "GS_PROJECT_MODULE/idl/kitex-grpc/kitex_gen/svc"
	"GS_PROJECT_MODULE/internal/api/server/kitexgrpcsvr/middleware"

	"go-spring.org/spring/gs"
)

func init() {
	// Register the Kitex gRPC/protobuf server and bind it to the Go-Spring
	// server lifecycle. Config is filled from the ${spring.kitex.grpc.server}
	// prefix; the server only materializes when a svc.GS_PROJECT_NAMEService
	// bean exists, mirroring the dubbo/grpc starters that gate on their
	// generated service interface.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.kitex.grpc.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[svc.GS_PROJECT_NAMEService]())
}

// Config defines Kitex gRPC/protobuf server configuration, bound from
// ${spring.kitex.grpc.server}.
type Config struct {
	Addr           string                    `value:"${addr:=:9095}"`
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a Kitex gRPC/protobuf server to the Go-Spring server lifecycle.
// It owns a done channel so Run can park until Stop signals shutdown, matching
// how the dubbo/grpc adapters manage their goroutine-blocking Serve calls.
type Server struct {
	cfg     Config
	handler svc.GS_PROJECT_NAMEService
	done    chan struct{}
}

// NewServer creates a Kitex gRPC/protobuf Server from
// ${spring.kitex.grpc.server} config and the registered
// svc.GS_PROJECT_NAMEService bean.
func NewServer(cfg Config, handler svc.GS_PROJECT_NAMEService) *Server {
	return &Server{cfg: cfg, handler: handler, done: make(chan struct{})}
}

// Run builds the Kitex gRPC/protobuf server on the configured address,
// registers the generated service against s.handler, then starts serving once
// Go-Spring signals readiness. Kitex's Run blocks forever internally, so it
// runs in a goroutine while Run parks on the done channel; Stop closes done to
// hand control back to Go-Spring's shutdown sequence.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	// TODO: resolve a TCP addr from s.cfg.Addr and build the Kitex gRPC server
	// with the endpoint middleware chain, backed by s.handler as the service
	// implementation, e.g.:
	//   addr, _ := net.ResolveTCPAddr("tcp", s.cfg.Addr)
	//   svr := gs_project_nameservice.NewServer(s.handler,
	//       server.WithServiceAddr(addr),
	//       server.WithPayloadCodec(protobuf.NewProtobufCodec()),
	//       server.WithMiddleware(middleware.Chain(
	//           middleware.Recovery(s.cfg.RecoveryConfig),
	//           middleware.Trace(s.cfg.TraceConfig),
	//           middleware.Metric(s.cfg.MetricConfig),
	//       )),
	//   )

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// TODO: errCh <- svr.Run() — Kitex's Run blocks until Stop is called.
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
	// TODO: svr.Stop() to trigger the underlying Kitex graceful shutdown.
	close(s.done)
	return nil
}
