// Package kratoswssvr wires kratos-WebSocket connection handlers into a single
// controller and holds the connection-loop logic. It uses the payload types
// from idl/kratos-ws/pb to decode envelope frames.
package kratoswssvr

import (
	"encoding/json"

	"GS_PROJECT_MODULE/idl/kratos-ws/pb"

	"github.com/gorilla/websocket"
	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(&GS_PROJECT_NAMEWebsocketController{})
}

// GS_PROJECT_NAMEWebsocketController owns the kratos-WebSocket connection
// handlers. Unlike the RPC servers there is no protocol-agnostic business
// entrypoint per method here; the loop decodes the wire envelope and dispatches
// on Envelope.Type.
type GS_PROJECT_NAMEWebsocketController struct{}

// Echo reads each envelope from the connection, treats the payload as an
// EchoPayload, and writes it back unchanged until the peer closes the
// connection or an error occurs. Replace this loop with real dispatch: switch
// on env.Type, decode the correct payload, call an embedded controller method,
// and write the encoded response back.
func (c *GS_PROJECT_NAMEWebsocketController) Echo(conn *websocket.Conn) {
	defer conn.Close()
	for {
		mt, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}

		// Decode the wire envelope. Non-JSON frames are echoed back verbatim so
		// simple clients still work; typed clients get an Envelope round-trip.
		var env pb.Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			if err = conn.WriteMessage(mt, raw); err != nil {
				return
			}
			continue
		}

		// TODO: switch on env.Type once more command payload types are added.
		var payload pb.EchoPayload
		if len(env.Payload) > 0 {
			_ = json.Unmarshal(env.Payload, &payload)
		}
		respPayload, _ := json.Marshal(&payload)
		resp, _ := json.Marshal(&pb.Envelope{Type: env.Type, Payload: respPayload})

		if err = conn.WriteMessage(mt, resp); err != nil {
			return
		}
	}
}
