// Package wssvr bootstraps a dedicated WebSocket server and binds it to the
// Go-Spring server lifecycle. It owns its own http.Server on an independent
// port (${spring.websocket.server}) instead of sharing the core gs.HttpServeMux,
// so the WebSocket endpoints listen separately from the plain HTTP server. The
// *websocket.Upgrader is contributed by starter-websocket; this package only
// mounts the upgrade routes and drives startup/graceful shutdown.
package wssvr

import (
	"context"
	"errors"
	"net"
	"net/http"

	"GS_PROJECT_MODULE/internal/api/server/wssvr/middleware"

	"github.com/gorilla/websocket"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

func init() {
	// Register the WebSocket server and bind it to the Go-Spring server
	// lifecycle. Config is filled from ${spring.websocket.server}; the server
	// only materialises when the WebSocket controller bean has been provided,
	// mirroring how the other transport servers gate on their handler bean.
	gs.Provide(NewServer, gs.IndexArg(0, gs.TagArg("${spring.websocket.server}"))).
		Export(gs.As[gs.Server]()).
		Condition(gs.OnBean[*GS_PROJECT_NAMEWebsocketController]())
}

// Config defines the WebSocket server configuration, bound from
// ${spring.websocket.server}. Addr is the independent listen address; the
// handshake middleware chain is tuned via the sub-configs. Upgrader tuning
// (buffer sizes, handshake timeout) lives under ${spring.websocket}.
type Config struct {
	Addr           string                    `value:"${addr:=:8010}"`
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

// Server adapts a WebSocket-capable http.Server to the Go-Spring server
// lifecycle. It listens on its own address so the WebSocket endpoints do not
// share a port with the plain HTTP server.
type Server struct {
	cfg      Config
	ctrl     *GS_PROJECT_NAMEWebsocketController
	upgrader *websocket.Upgrader
	svr      *http.Server
}

// NewServer creates a WebSocket Server from ${spring.websocket.server} config,
// the composed WebSocket controller bean, and the shared *websocket.Upgrader
// contributed by starter-websocket.
func NewServer(cfg Config, ctrl *GS_PROJECT_NAMEWebsocketController, upgrader *websocket.Upgrader) *Server {
	return &Server{cfg: cfg, ctrl: ctrl, upgrader: upgrader}
}

// Run builds the WebSocket routes on the configured address and starts serving
// once Go-Spring signals readiness.
func (s *Server) Run(ctx context.Context, sig gs.ReadySignal) error {
	mux := http.NewServeMux()

	// upgrade promotes an HTTP request to a WebSocket connection and hands it
	// to the given connection handler through the handshake middleware chain.
	upgrade := func(serve func(*websocket.Conn)) http.Handler {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := s.upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Errorf(r.Context(), log.TagAppDef, "failed to upgrade websocket: %v", err)
				return
			}
			serve(conn)
		})
		return middleware.Chain(h,
			middleware.Recovery(s.cfg.RecoveryConfig),
			middleware.Trace(s.cfg.TraceConfig),
			middleware.Metric(s.cfg.MetricConfig),
		)
	}

	// Register all WebSocket endpoints here.
	mux.Handle("/ws/echo", upgrade(s.ctrl.Echo))

	s.svr = &http.Server{Addr: s.cfg.Addr, Handler: mux}

	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}

	<-sig.TriggerAndWait()

	if err = s.svr.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Stop gracefully shuts the WebSocket server down so Go-Spring can complete its
// shutdown sequence.
func (s *Server) Stop() error {
	if s.svr == nil {
		return nil
	}
	return s.svr.Shutdown(context.Background())
}
