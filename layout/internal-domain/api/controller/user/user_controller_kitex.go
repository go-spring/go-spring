// Package user also hosts the per-protocol controller adapters, distinguished by
// file suffix (user_controller_<proto>.go). This file is the Kitex adapter; see
// user_controller.go for the HTTP baseline. All adapters delegate to the same
// user application service, so every protocol converges on one core.
package user

import (
	"context"

	svc "GS_PROJECT_MODULE/idl-domain/kitex/kitex_gen/svc"
	"GS_PROJECT_MODULE/internal-domain/api/controller/user/converter"
	"GS_PROJECT_MODULE/internal-domain/application/user"
)

// KitexUserController adapts Kitex-generated user calls to the user
// application service.
type KitexUserController struct {
	UserService *user.UserService `autowire:""`
}

// RegisterUser converts the Kitex request to a DTO, delegates to UserService,
// and maps the result back to a Kitex UserResp.
func (c *KitexUserController) RegisterUser(ctx context.Context, req *svc.RegisterUserReq) (*svc.UserResp, error) {
	resp := &svc.UserResp{}
	u, err := c.UserService.RegisterUser(converter.FromKitexRegisterUserReq(req))
	if err != nil {
		resp.Code = 1
		resp.Message = err.Error()
		return resp, nil
	}
	resp.Data = converter.ToKitexUser(u, req.Email)
	return resp, nil
}

// UpgradeUser is a stub: the application service has no upgrade path yet, so
// this controller returns a "not implemented" response to keep the Kitex
// service surface complete.
func (c *KitexUserController) UpgradeUser(ctx context.Context, req *svc.UpgradeUserReq) (*svc.UserResp, error) {
	// TODO: wire to *user.UserService once an Upgrade method exists.
	return &svc.UserResp{Code: 1, Message: "UpgradeUser not implemented"}, nil
}
