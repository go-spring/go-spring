// Package goframetcpsvr also holds the per-connection handler. This mirrors
// wssvr: the connection loop lives in the server package (there is no
// controller matrix in api/controller), so the frame codec and dispatch are
// self-contained here.
package goframe_tcpsvr

import (
	"context"
	"net"

	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(&GS_PROJECT_NAMETCPController{})
}

// GS_PROJECT_NAMETCPController owns the per-connection dispatch loop. It maps
// pb.Frame requests to protocol-agnostic business entrypoints; the outer server
// only drives the accept loop and the middleware chain.
type GS_PROJECT_NAMETCPController struct{}

// Serve reads one frame at a time and echoes it back until the peer closes the
// connection or a read/write error occurs. Replace this loop with real
// dispatch: decode the pb.Frame, dispatch on Frame.Type, and write the encoded
// response back.
func (c *GS_PROJECT_NAMETCPController) Serve(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		if _, err = conn.Write(buf[:n]); err != nil {
			return
		}
	}
}
