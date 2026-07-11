// Package thriftsvr bootstraps the Thrift server and binds it to the Go-Spring
// server lifecycle. It mirrors httpsvr but adapts a Thrift server to gs.Server
// so the container drives startup and graceful shutdown.
package thriftsvr

import (
	"context"

	thrift "GS_PROJECT_MODULE/idl/thrift/gen"
	"GS_PROJECT_MODULE/internal/api/server/thriftsvr/middleware"

	"go-spring.org/spring/gs"
)

func init() {
	// Register the Thrift server and bind it to the Go-Spring server lifecycle.
	// Config is filled from the ${spring.thrift.server} prefix; the server only
	// materializes when a thrift.GS_PROJECT_NAMEService bean exists, mirroring the
	// dubbo/grpc/kitex starters that gate on their generated service interface.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.thrift.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[thrift.GS_PROJECT_NAMEService]())
}

// Config defines Thrift server configuration, bound from ${spring.thrift.server}.
type Config struct {
	Addr           string                    `value:"${addr:=:9092}"`
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a Thrift server to the Go-Spring server lifecycle. It owns a
// done channel so Run can park until Stop signals shutdown, matching how the
// dubbo/grpc/kitex adapters manage their goroutine-blocking Serve calls.
type Server struct {
	cfg     Config
	handler thrift.GS_PROJECT_NAMEService
	done    chan struct{}
}

// NewServer creates a Thrift Server from ${spring.thrift.server} config and the
// registered thrift.GS_PROJECT_NAMEService bean.
func NewServer(cfg Config, handler thrift.GS_PROJECT_NAMEService) *Server {
	return &Server{cfg: cfg, handler: handler, done: make(chan struct{})}
}

// Run builds the Thrift server on the configured address, registers the
// generated processor against s.handler, then starts serving once Go-Spring
// signals readiness. Thrift's Serve blocks forever internally, so it runs in a
// goroutine while Run parks on the done channel; Stop closes done to hand
// control back to Go-Spring's shutdown sequence.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	// TODO: build the Thrift processor with the middleware chain, backed by
	// s.handler as the service implementation, e.g.:
	//   processor := thrift.NewGS_PROJECT_NAMEServiceProcessor(s.handler)
	//   transport, _ := thrift.NewTServerSocket(s.cfg.Addr)
	//   srv := thrift.NewTSimpleServer4(processor, transport,
	//       thrift.NewTTransportFactory(), thrift.NewTBinaryProtocolFactoryConf(nil))

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// TODO: errCh <- srv.Serve() — Thrift's Serve blocks until Stop is called.
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
	// TODO: srv.Stop() to trigger the underlying Thrift graceful shutdown.
	close(s.done)
	return nil
}
