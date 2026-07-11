// Package dubbosvr bootstraps the Dubbo (triple) server and binds it to the
// Go-Spring server lifecycle. It mirrors httpsvr but adapts a Dubbo server to
// gs.Server so the container drives startup and graceful shutdown.
package dubbosvr

import (
	"context"

	"GS_PROJECT_MODULE/idl/dubbo/triple"
	"GS_PROJECT_MODULE/internal/api/server/dubbosvr/middleware"

	"go-spring.org/spring/gs"
)

func init() {
	// Register the Dubbo (triple) server and bind it to the Go-Spring server
	// lifecycle. Config is filled from the ${spring.dubbo.server} prefix; the
	// server only materializes when a triple.GS_PROJECT_NAMEServiceHandler bean
	// exists, mirroring how the thrift/grpc starters gate on their processor
	// bean.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.dubbo.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[triple.GS_PROJECT_NAMEServiceHandler]())
}

// Config defines Dubbo (triple) server configuration, bound from
// ${spring.dubbo.server}.
type Config struct {
	Port           int                       `value:"${port:=20000}"`
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a Dubbo (triple) server to the Go-Spring server lifecycle.
// Dubbo's Serve blocks forever internally, so Run parks on a done channel while
// serve runs in a goroutine; Stop closes done to hand control back to
// Go-Spring's shutdown sequence.
type Server struct {
	cfg     Config
	handler triple.GS_PROJECT_NAMEServiceHandler
	done    chan struct{}
}

// NewServer creates a Dubbo (triple) Server from ${spring.dubbo.server} config
// and the registered triple service handler bean.
func NewServer(cfg Config, handler triple.GS_PROJECT_NAMEServiceHandler) *Server {
	return &Server{cfg: cfg, handler: handler, done: make(chan struct{})}
}

// Run builds the Dubbo (triple) server on the configured port, registers the
// generated service against s.handler, then starts serving once Go-Spring
// signals readiness.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	// TODO: build the Dubbo server with the filter middleware chain and
	// register the generated service handler, e.g.:
	//   svr, err := server.NewServer(
	//       server.WithServerProtocol(
	//           protocol.WithPort(s.cfg.Port),
	//           protocol.WithTriple(),
	//       ),
	//   )
	//   if err != nil { return err }
	//   if err := triple.RegisterGS_PROJECT_NAMEServiceHandler(svr, s.handler); err != nil {
	//       return err
	//   }
	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// TODO: svr.Serve() exports the service (binding the listener on
		// s.cfg.Port) and then blocks until Stop is called.
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
	close(s.done)
	return nil
}
