// Package wssvr wires WebSocket connection handlers into a single controller
// and holds the connection-loop logic that adapts frames to the application layer.
package wssvr

import (
	"github.com/gorilla/websocket"
	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(&GS_PROJECT_NAMEWebsocketController{})
}

// GS_PROJECT_NAMEWebsocketController composes order and user controllers so
// connection handlers can delegate to protocol-agnostic business entrypoints.
type GS_PROJECT_NAMEWebsocketController struct{}

// Echo reads each message from the connection and writes it back unchanged
// until the peer closes the connection or an error occurs. Replace this loop
// with real dispatch: decode the frame, call an embedded controller method,
// and write the encoded response back.
func (c *GS_PROJECT_NAMEWebsocketController) Echo(conn *websocket.Conn) {
	defer conn.Close()
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err = conn.WriteMessage(mt, msg); err != nil {
			return
		}
	}
}
