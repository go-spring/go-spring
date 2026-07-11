// Package wssvr registers WebSocket server components and wires them into the application.
package wssvr

import (
	"net/http"

	"GS_PROJECT_MODULE/internal/api/server/wssvr/middleware"

	"github.com/gorilla/websocket"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterWebsocket "go-spring.org/starter-websocket"
)

// ServerConfig defines the configuration for the WebSocket handshake middleware
// chain. Upgrader tuning (buffer sizes, handshake timeout) and the listen
// address live in the starter under ${spring.websocket.server}.
type ServerConfig struct {
	RecoveryConfig middleware.RecoveryConfig `value:"${recovery}"`
	TraceConfig    middleware.TraceConfig    `value:"${trace}"`
	MetricConfig   middleware.MetricConfig   `value:"${metric}"`
}

func init() {
	// Provide a ServerRegister so the websocket starter exposes upgrade
	// endpoints. The starter's SimpleWebsocketServer is only registered when
	// this bean is present.
	gs.Provide(func(config ServerConfig, server *GS_PROJECT_NAMEWebsocketController) StarterWebsocket.ServerRegister {
		return func(mux *http.ServeMux, upgrader *websocket.Upgrader) {
			// upgrade promotes an HTTP request to a WebSocket connection and
			// hands it to the given connection handler.
			upgrade := func(serve func(*websocket.Conn)) http.Handler {
				h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					conn, err := upgrader.Upgrade(w, r, nil)
					if err != nil {
						log.Errorf(r.Context(), log.TagAppDef, "failed to upgrade websocket: %v", err)
						return
					}
					serve(conn)
				})
				return middleware.Chain(h,
					middleware.Recovery(config.RecoveryConfig),
					middleware.Trace(config.TraceConfig),
					middleware.Metric(config.MetricConfig),
				)
			}

			// Register all WebSocket endpoints here.
			mux.Handle("/ws/echo", upgrade(server.Echo))
		}
	})
}
