// Package httpsvr wires HTTP handlers into a single controller and registers the server.
package httpsvr

import (
	"context"

	"GS_PROJECT_MODULE/idl-domain/http/proto"
	orderCtrl "GS_PROJECT_MODULE/internal-domain/api/controller/order"
	userCtrl "GS_PROJECT_MODULE/internal-domain/api/controller/user"

	"go-spring.org/spring/gs"
)

func init() {
	gs.Provide(&GS_PROJECT_NAMEController{})
}

// GS_PROJECT_NAMEController composes order and user controllers into a single
// controller that satisfies the GS_PROJECT_NAMEServer interface.
type GS_PROJECT_NAMEController struct {
	orderCtrl.OrderController
	userCtrl.UserController
}

// Ping is kept at the protocol layer as a simple health check endpoint.
func (c *GS_PROJECT_NAMEController) Ping(ctx context.Context, req *proto.PingReq) *proto.PingResp {
	return &proto.PingResp{
		Errno:  proto.ErrCode_ErrOk,
		Errmsg: proto.ErrCode_name[proto.ErrCode_ErrOk],
		Data:   "pong",
	}
}