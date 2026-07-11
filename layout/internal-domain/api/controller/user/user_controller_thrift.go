// Package user also hosts the per-protocol controller adapters, distinguished by
// file suffix (user_controller_<proto>.go). This file is the Thrift adapter; see
// user_controller.go for the HTTP baseline. All adapters delegate to the same
// user application service, so every protocol converges on one core.
package user

import (
	"context"

	thrift "GS_PROJECT_MODULE/idl-domain/thrift/gen"
	"GS_PROJECT_MODULE/internal-domain/api/controller/user/converter"
	"GS_PROJECT_MODULE/internal-domain/application/user"
)

// ThriftUserController adapts Thrift-generated user calls to the user
// application service.
type ThriftUserController struct {
	UserService *user.UserService `autowire:""`
}

// RegisterUser converts the Thrift request to a DTO, delegates to UserService,
// and maps the result back to a Thrift UserResp.
func (c *ThriftUserController) RegisterUser(ctx context.Context, req *thrift.RegisterUserReq) (*thrift.UserResp, error) {
	resp := &thrift.UserResp{}
	u, err := c.UserService.RegisterUser(converter.FromThriftRegisterUserReq(req))
	if err != nil {
		resp.Code = 1
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = converter.ToThriftUser(u, req.Email)
	return resp, nil
}

// UpgradeUser is a stub: the application service has no upgrade path yet, so
// this controller returns a "not implemented" response to keep the Thrift
// service surface complete.
func (c *ThriftUserController) UpgradeUser(ctx context.Context, req *thrift.UpgradeUserReq) (*thrift.UserResp, error) {
	// TODO: wire to *user.UserService once an Upgrade method exists.
	return &thrift.UserResp{Code: 1, Message: "UpgradeUser not implemented"}, nil
}
