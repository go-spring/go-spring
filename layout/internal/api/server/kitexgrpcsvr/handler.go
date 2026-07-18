// Package kitexgrpcsvr wires Kitex gRPC/protobuf handlers into a single
// controller and registers it.
package kitexgrpcsvr

import (
	"context"

	svc "GS_PROJECT_MODULE/idl/kitex-grpc/kitex_gen/svc"
	orderCtrl "GS_PROJECT_MODULE/internal/api/controller/order"
	userCtrl "GS_PROJECT_MODULE/internal/api/controller/user"

	"go-spring.org/spring/gs"
)

func init() {
	// Export the composed controller as svc.GS_PROJECT_NAMEService. The server
	// bean (see kitexgrpcsvr.go) gates on this interface bean, mirroring how the
	// dubbo/grpc starters wire a generated service handler.
	gs.Provide(&GS_PROJECT_NAMEController{}).Export(gs.As[svc.GS_PROJECT_NAMEService]())
}

// GS_PROJECT_NAMEController composes the Kitex gRPC/protobuf order and user
// controllers into a single controller that satisfies the generated
// svc.GS_PROJECT_NAMEService interface. The embedded controllers live in
// api/controller and adapt Kitex request/response types to application DTOs.
type GS_PROJECT_NAMEController struct {
	orderCtrl.KitexGrpcOrderController
	userCtrl.KitexGrpcUserController
}

// Ping is kept at the protocol layer as a simple health check endpoint. The
// order and user controllers deliberately do not implement Ping.
func (c *GS_PROJECT_NAMEController) Ping(ctx context.Context, req *svc.PingReq) (*svc.PingResp, error) {
	return &svc.PingResp{Message: "pong"}, nil
}
