// Package thriftsvr wires Thrift handlers into a single controller and registers it.
package thriftsvr

import (
	"context"

	thrift "GS_PROJECT_MODULE/idl-domain/thrift/gen"
	orderCtrl "GS_PROJECT_MODULE/internal-domain/api/controller/order"
	userCtrl "GS_PROJECT_MODULE/internal-domain/api/controller/user"

	"go-spring.org/spring/gs"
)

func init() {
	// Export the composed controller as thrift.GS_PROJECT_NAMEService so the
	// thrift server adapter (see thriftsvr.go) can gate on it and register it as
	// the processor backend.
	gs.Provide(&GS_PROJECT_NAMEController{}).Export(gs.As[thrift.GS_PROJECT_NAMEService]())
}

// GS_PROJECT_NAMEController composes the Thrift order and user controllers into a
// single controller that satisfies the generated thrift.GS_PROJECT_NAMEService
// interface. The embedded controllers live in api/controller and adapt Thrift
// request/response types to application DTOs.
type GS_PROJECT_NAMEController struct {
	orderCtrl.ThriftOrderController
	userCtrl.ThriftUserController
}

// Ping is kept at the protocol layer as a simple health check endpoint.
func (c *GS_PROJECT_NAMEController) Ping(ctx context.Context, req *thrift.PingReq) (*thrift.PingResp, error) {
	return &thrift.PingResp{Message: "pong"}, nil
}
