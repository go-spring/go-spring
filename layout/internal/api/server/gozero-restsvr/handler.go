// Package gozerosvr wires go-zero handlers into a single controller and
// exports it as the GS_PROJECT_NAMELogic bean that the server gates on.
package gozero_restsvr

import (
	"context"

	"GS_PROJECT_MODULE/idl/gozero-rest/handler"
	types "GS_PROJECT_MODULE/idl/gozero-rest/types"
	orderCtrl "GS_PROJECT_MODULE/internal/api/controller/order"
	userCtrl "GS_PROJECT_MODULE/internal/api/controller/user"

	"go-spring.org/spring/gs"
)

func init() {
	// Register the composed controller as a handler.GS_PROJECT_NAMELogic bean.
	// The go-zero server (see gozerosvr.go) declares a dependency on this
	// interface, so exporting it here is what wires the goctl-generated routes
	// into Go-Spring's IoC container instead of a hand-built main().
	gs.Provide(&GS_PROJECT_NAMEController{}).
		Export(gs.As[handler.GS_PROJECT_NAMELogic]())
}

// GS_PROJECT_NAMEController composes the go-zero order and user controllers
// into a single value that satisfies handler.GS_PROJECT_NAMELogic through
// embedding. Ping stays here at the protocol layer as a trivial health check.
type GS_PROJECT_NAMEController struct {
	orderCtrl.GozeroOrderController
	userCtrl.GozeroUserController
}

// Ping is kept at the protocol layer as a simple health-check endpoint.
func (c *GS_PROJECT_NAMEController) Ping(ctx context.Context, req *types.PingReq) (*types.PingResp, error) {
	return &types.PingResp{Message: "pong"}, nil
}
