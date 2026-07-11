// Package dubbosvr wires Dubbo (triple) controllers into a single composed
// controller and registers it as the triple service handler bean.
package dubbosvr

import (
	"context"

	"GS_PROJECT_MODULE/idl-domain/dubbo/triple"
	orderCtrl "GS_PROJECT_MODULE/internal-domain/api/controller/order"
	userCtrl "GS_PROJECT_MODULE/internal-domain/api/controller/user"

	"go-spring.org/spring/gs"
)

func init() {
	// Provide the composed controller and export it as the triple service
	// handler. dubbosvr.NewServer depends on this interface, so the Condition
	// on the server bean triggers registration only after the handler exists.
	gs.Provide(&GS_PROJECT_NAMEController{}).
		Export(gs.As[triple.GS_PROJECT_NAMEServiceHandler]())
}

// GS_PROJECT_NAMEController composes the Dubbo order and user controllers into
// a single controller that satisfies triple.GS_PROJECT_NAMEServiceHandler. The
// embedded controllers live in api/controller and adapt triple
// request/response types to application DTOs.
type GS_PROJECT_NAMEController struct {
	orderCtrl.DubboOrderController
	userCtrl.DubboUserController
}

// Ping is kept at the protocol layer as a simple health check endpoint.
func (c *GS_PROJECT_NAMEController) Ping(ctx context.Context, req *triple.PingReq) (*triple.PingResp, error) {
	return &triple.PingResp{Message: "pong"}, nil
}
