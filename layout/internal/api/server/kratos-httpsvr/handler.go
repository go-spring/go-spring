// Package kratoshttpsvr wires kratos-HTTP handlers into a single controller and
// registers it as the pb.GS_PROJECT_NAMEServer bean the server adapter depends
// on.
package kratos_httpsvr

import (
	"context"

	"GS_PROJECT_MODULE/idl/kratos-http/pb"
	orderCtrl "GS_PROJECT_MODULE/internal/api/controller/order"
	userCtrl "GS_PROJECT_MODULE/internal/api/controller/user"

	"go-spring.org/spring/gs"
)

func init() {
	// Export the composed controller as the generated kratos-HTTP server
	// interface so the server adapter can register it against the kratos
	// HTTP server in Run().
	gs.Provide(&GS_PROJECT_NAMEController{}).Export(gs.As[pb.GS_PROJECT_NAMEServer]())
}

// GS_PROJECT_NAMEController composes the per-domain kratos-HTTP controllers
// into a single value that satisfies pb.GS_PROJECT_NAMEServer. The embedded
// controllers live in api/controller and adapt pb request/response types to
// application DTOs; the outer type only owns rpcs that are cross-domain (Ping).
type GS_PROJECT_NAMEController struct {
	orderCtrl.KratosHttpOrderController
	userCtrl.KratosHttpUserController
}

// Ping is the protocol-layer health check; it does not touch any application
// service and returns a fixed pong body.
func (c *GS_PROJECT_NAMEController) Ping(ctx context.Context, req *pb.PingReq) (*pb.PingResp, error) {
	return &pb.PingResp{Message: "pong"}, nil
}
