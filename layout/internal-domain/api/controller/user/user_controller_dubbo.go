// Package user also hosts the per-protocol controller adapters, distinguished
// by file suffix (user_controller_<proto>.go). This file is the Dubbo (triple)
// adapter; see user_controller.go for the HTTP baseline. All adapters delegate
// to the same user application service, so every protocol converges on one
// core.
package user

import (
	"context"

	"GS_PROJECT_MODULE/idl-domain/dubbo/triple"
	"GS_PROJECT_MODULE/internal-domain/api/controller/user/converter"
	"GS_PROJECT_MODULE/internal-domain/application/user"
)

// DubboUserController adapts Dubbo (triple) user calls to the user application
// service. It consumes the triple-generated request/response types directly and
// converts to/from application DTOs at the boundary.
type DubboUserController struct {
	UserService *user.UserService `autowire:""`
}

// RegisterUser converts a triple RegisterUserReq into a DTO, delegates to the
// application service, and maps the result back to a triple UserResp.
func (c *DubboUserController) RegisterUser(ctx context.Context, req *triple.RegisterUserReq) (*triple.UserResp, error) {
	u, err := c.UserService.RegisterUser(converter.FromDubboRegisterUserReq(req))
	if err != nil {
		return &triple.UserResp{Code: 1, Message: err.Error()}, nil
	}
	return &triple.UserResp{Code: 0, Message: "ok", Data: converter.ToDubboUser(u, req.GetEmail())}, nil
}

// UpgradeUser upgrades an existing user's privileges (stub).
func (c *DubboUserController) UpgradeUser(ctx context.Context, req *triple.UpgradeUserReq) (*triple.UserResp, error) {
	return &triple.UserResp{Code: 0, Message: "ok"}, nil
}
