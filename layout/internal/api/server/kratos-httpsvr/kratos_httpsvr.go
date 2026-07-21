// Package kratos_httpsvr bootstraps the kratos-HTTP server and binds it to the
// Go-Spring server lifecycle. It mirrors grpcsvr but targets kratos's HTTP
// transport so the container drives startup and graceful shutdown; kratos HTTP
// runs on its own port independent of the plain HTTP server.
package kratos_httpsvr

import (
	"context"

	"GS_PROJECT_MODULE/idl/kratos-http/pb"
	"GS_PROJECT_MODULE/internal/api/server/kratos-httpsvr/middleware"

	"go-spring.org/spring/gs"
)

func init() {
	// Register the kratos-HTTP server and bind it to the Go-Spring server
	// lifecycle. Config is filled from the ${spring.kratos.http.server} prefix;
	// the server only materialises when a pb.GS_PROJECT_NAMEServer bean has
	// been exported, mirroring how the grpc / dubbo / thrift starters gate on
	// their handler bean.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.kratos.http.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[pb.GS_PROJECT_NAMEServer]())
}

// Config defines kratos-HTTP server configuration, bound from
// ${spring.kratos.http.server}.
type Config struct {
	Addr           string                    `value:"${addr:=:8005}"`
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a kratos-HTTP server to the Go-Spring server lifecycle. The
// underlying kratos transport blocks in Start(), so Run offloads it to a
// goroutine and parks on a done channel; Stop closes done to hand control back
// to Go-Spring's shutdown sequence.
type Server struct {
	cfg  Config
	ctrl *GS_PROJECT_NAMEController
	done chan struct{}
}

// NewServer creates a kratos-HTTP Server from ${spring.kratos.http.server}
// config and the composed controller bean.
func NewServer(cfg Config, ctrl *GS_PROJECT_NAMEController) *Server {
	return &Server{cfg: cfg, ctrl: ctrl, done: make(chan struct{})}
}

// Run builds the kratos-HTTP server on the configured address, registers the
// generated routes against s.ctrl, then starts serving once Go-Spring signals
// readiness.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	// TODO: build the kratos-HTTP server with the middleware chain and
	// register the generated routes backed by s.ctrl, e.g.:
	//   srv := kratoshttp.NewServer(
	//       kratoshttp.Address(s.cfg.Addr),
	//       kratoshttp.Middleware(
	//           middleware.Recovery(s.cfg.RecoveryConfig),
	//           middleware.Trace(s.cfg.TraceConfig),
	//           middleware.Metric(s.cfg.MetricConfig),
	//       ),
	//   )
	//   pb.RegisterGS_PROJECT_NAMEHTTPServer(srv, s.ctrl)

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// TODO: errCh <- srv.Start(ctx) — kratos Server.Start blocks until Stop.
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
	// TODO: srv.Stop(context.Background())
	close(s.done)
	return nil
}
