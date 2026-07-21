// Package goframewssvr wires goframe WebSocket connection handlers into a
// single controller and holds the connection-loop logic that adapts frames to
// the application layer.
package goframe_wssvr

import (
	"github.com/gorilla/websocket"
	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(&GS_PROJECT_NAMEWebsocketController{})
}

// GS_PROJECT_NAMEWebsocketController owns the per-connection dispatch loop for
// the goframe WebSocket server. There is no controller matrix in
// api/controller — the frame codec and dispatch are self-contained here.
type GS_PROJECT_NAMEWebsocketController struct{}

// Echo reads each message from the connection and writes it back unchanged
// until the peer closes the connection or an error occurs. Replace this loop
// with real dispatch: decode the pb.Frame, dispatch on Frame.Type, and write
// the encoded response back.
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
