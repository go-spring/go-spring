// Package gozerosvr bootstraps a goctl-style HTTP server and binds it to the
// Go-Spring server lifecycle. The generated types + routes live under
// idl/gozero; this file only owns the gs.Server adapter so the container
// drives startup and graceful shutdown.
package gozerosvr

import (
	"context"
	"fmt"
	"net/http"

	"GS_PROJECT_MODULE/idl/gozero/handler"
	"GS_PROJECT_MODULE/internal/api/server/gozerosvr/middleware"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register the go-zero server and bind it to the Go-Spring server lifecycle.
	// Config is filled from the ${spring.gozero.server} prefix. The server only
	// materializes when a handler.GS_PROJECT_NAMELogic bean exists, mirroring
	// how the thrift/grpc starters gate on their processor bean.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.gozero.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[handler.GS_PROJECT_NAMELogic]())
}

// Config defines go-zero rest server configuration, bound from
// ${spring.gozero.server}. Fields mirror go-zero's rest.RestConf naming so the
// bound config can be handed directly to rest.MustNewServer once the
// integration is filled in.
type Config struct {
	Name string `value:"${name:=GS_PROJECT_NAME}"`
	Host string `value:"${host:=0.0.0.0}"`
	Port int    `value:"${port:=8899}"`

	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a goctl-style HTTP server to the Go-Spring server lifecycle.
// httpSvr is captured so Stop can perform a graceful Shutdown; done backs the
// same "block in Run, close in Stop" pattern used by dubbosvr/thriftsvr, so the
// container drives shutdown ordering.
type Server struct {
	cfg     Config
	logic   handler.GS_PROJECT_NAMELogic
	httpSvr *http.Server
	done    chan struct{}
}

// NewServer creates a go-zero Server from ${spring.gozero.server} config and
// the exported logic bean.
func NewServer(cfg Config, logic handler.GS_PROJECT_NAMELogic) *Server {
	return &Server{cfg: cfg, logic: logic, done: make(chan struct{})}
}

// Run builds the goctl-generated routes, wraps them in the middleware chain,
// then starts serving once Go-Spring signals readiness. ListenAndServe blocks,
// so it runs in a goroutine while Run parks on the done channel; Stop closes
// done to hand control back to Go-Spring's shutdown sequence.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	mux := http.NewServeMux()
	handler.RegisterHandlers(mux, s.logic)

	root := middleware.Chain(
		mux,
		middleware.Recovery(s.cfg.RecoveryConfig),
		middleware.Trace(s.cfg.TraceConfig),
		middleware.Metric(s.cfg.MetricConfig),
	)

	s.httpSvr = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port),
		Handler: root,
	}

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// TODO: swap for rest.MustNewServer(...).Start() once the go-zero
		// dependency is pulled in; net/http keeps the template compilable.
		errCh <- s.httpSvr.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return errutil.Explain(err, "failed to serve on %s", s.httpSvr.Addr)
	case <-s.done:
		return nil
	}
}

// Stop gracefully shuts the HTTP server down and signals Run to return so
// Go-Spring can complete its shutdown sequence. The explicit Shutdown is what
// actually unblocks ListenAndServe; closing done releases Run's select.
func (s *Server) Stop() error {
	if s.httpSvr != nil {
		_ = s.httpSvr.Shutdown(context.Background())
	}
	close(s.done)
	return nil
}
